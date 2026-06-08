// Package host implements the remotty host daemon that runs on machines
// to be accessed remotely. It connects to the signaling server and
// manages incoming WebRTC sessions.
package host

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/rs/zerolog"
	"github.com/sametyilmaztemel/remotty/internal/auth"
	"github.com/sametyilmaztemel/remotty/internal/config"
	"github.com/sametyilmaztemel/remotty/internal/protocol"
	"github.com/sametyilmaztemel/remotty/internal/pty"
	"github.com/sametyilmaztemel/remotty/internal/screen"
	"github.com/sametyilmaztemel/remotty/internal/transfer"
	"github.com/sametyilmaztemel/remotty/internal/webrtc"
)

// Daemon runs on the host machine and manages remote access sessions.
type Daemon struct {
	cfg        config.HostConfig
	signalConn *websocket.Conn
	signalMu   sync.Mutex // guards concurrent writes to signalConn
	peerID     string
	webrtcEng  *webrtc.Engine
	ptyMgr     *pty.Manager
	transferMgr *transfer.Manager
	sessions   map[string]*Session
	mu         sync.RWMutex
	done       chan struct{}
	log        zerolog.Logger
	clipMon    *ClipboardMonitor

	localAPI     *http.Server
	localAPIOnce sync.Once
}

// Session tracks an active client connection.
type Session struct {
	ID             string
	ClientID       string
	RoomID         string
	WebRTC         *webrtc.Engine
	PTYSess        *pty.Session
	ScreenStreamer *screen.Streamer
	CreatedAt      time.Time
	Authed         bool
	// Reconnect state
	reconnecting bool
	disconnected bool
	mu           sync.Mutex
}

// NewDaemon creates a new host daemon.
func NewDaemon(cfg config.HostConfig, log zerolog.Logger) (*Daemon, error) {
	// Hash master password if provided in plaintext
	if cfg.MasterPassword != "" && cfg.MasterHash == "" {
		hash, err := auth.HashPassword(cfg.MasterPassword)
		if err != nil {
			return nil, fmt.Errorf("hash master password: %w", err)
		}
		cfg.MasterHash = hash
	}

	// Security check: warn if no auth is configured
	if cfg.MasterHash == "" && cfg.MasterPassword == "" {
		log.Warn().Msg("No master password configured — anyone can connect!")
	}
	if cfg.RequireAuth && cfg.MasterHash == "" && cfg.MasterPassword == "" {
		return nil, fmt.Errorf("require_auth is enabled but no master_password or master_hash is set")
	}

	if cfg.Name == "" {
		cfg.Name, _ = os.Hostname()
	}
	if cfg.Features == nil {
		cfg.Features = []string{"terminal"}
		if runtime.GOOS == "darwin" {
			cfg.Features = append(cfg.Features, "screen")
		}
	}
	if cfg.ReconnectWait == 0 {
		cfg.ReconnectWait = 5 * time.Second
	}
	if cfg.HeartbeatInt == 0 {
		cfg.HeartbeatInt = 15 * time.Second
	}
	if cfg.MaxSessions == 0 {
		cfg.MaxSessions = 10
	}

	dataDir := "$HOME/.remotty"
	if home, err := os.UserHomeDir(); err == nil {
		dataDir = filepath.Join(home, ".remotty")
	}

	return &Daemon{
		cfg:         cfg,
		ptyMgr:     pty.NewManager(),
		transferMgr: transfer.NewManager(dataDir),
		sessions:    make(map[string]*Session),
		done:        make(chan struct{}),
		log:         log.With().Str("component", "host").Logger(),
	}, nil
}

// Run starts the host daemon and blocks until context cancellation.
func (d *Daemon) Run(ctx context.Context) error {
	defer d.cleanup()

	// Start local HTTP API for macOS menu bar app
	d.startLocalAPI()

	backoff := d.cfg.ReconnectWait
	maxBackoff := d.cfg.ReconnectMaxWait
	if maxBackoff < backoff {
		maxBackoff = backoff
	}

	for {
		select {
		case <-ctx.Done():
			return nil
		default:
			if err := d.connect(ctx); err != nil {
				d.log.Error().
					Err(err).
					Dur("retry_after", backoff).
					Msg("Connection failed, reconnecting...")
				select {
				case <-ctx.Done():
					return nil
				case <-time.After(backoff):
					// Exponential backoff with max cap
					backoff = backoff * 2
					if backoff > maxBackoff {
						backoff = maxBackoff
					}
				}
			} else {
				// Reset backoff on successful connection
				backoff = d.cfg.ReconnectWait
			}
		}
	}
}

func (d *Daemon) connect(ctx context.Context) error {
	d.log.Info().Str("url", d.cfg.SignalURL+"/ws").Msg("Connecting to signaling server")

	conn, _, err := websocket.DefaultDialer.DialContext(ctx, d.cfg.SignalURL+"/ws", nil)
	if err != nil {
		return fmt.Errorf("dial: %w", err)
	}

	d.mu.Lock()
	d.signalConn = conn
	d.mu.Unlock()

	defer conn.Close()

	// Register as host
	regMsg := protocol.NewMessage(protocol.MsgRegister, protocol.RegisterPayload{
		Name:     d.cfg.Name,
		Platform: runtime.GOOS,
		Arch:     runtime.GOARCH,
		Version:  config.Version,
		Features: d.cfg.Features,
		DeviceID: d.cfg.DeviceID,
	})

	if err := conn.WriteJSON(regMsg); err != nil {
		return fmt.Errorf("register: %w", err)
	}

	// Read registration response
	_, data, err := conn.ReadMessage()
	if err != nil {
		return fmt.Errorf("read register response: %w", err)
	}

	var resp protocol.Message
	json.Unmarshal(data, &resp)
	if resp.Type == protocol.MsgRegister {
		var payload map[string]interface{}
		json.Unmarshal(resp.Payload, &payload)
		d.peerID, _ = payload["id"].(string)
		d.log.Info().Str("peer_id", d.peerID).Msg("Registered with signaling server")

		// Call OnRegistered callback if set
		if d.cfg.OnRegistered != nil {
			d.cfg.OnRegistered(d.peerID)
		}
	}

	// Start heartbeat
	hbCtx, hbCancel := context.WithCancel(ctx)
	defer hbCancel()
	go d.heartbeatLoop(hbCtx, conn)

	// Read loop
	return d.readLoop(ctx, conn)
}

func (d *Daemon) heartbeatLoop(ctx context.Context, conn *websocket.Conn) {
	ticker := time.NewTicker(d.cfg.HeartbeatInt)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := conn.WriteJSON(protocol.NewMessage(protocol.MsgHeartbeat, nil)); err != nil {
				d.log.Warn().Err(err).Msg("Heartbeat failed")
				return
			}
		}
	}
}

func (d *Daemon) readLoop(ctx context.Context, conn *websocket.Conn) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		default:
			_, data, err := conn.ReadMessage()
			if err != nil {
				return fmt.Errorf("read: %w", err)
			}

			var msg protocol.Message
			if err := json.Unmarshal(data, &msg); err != nil {
				d.log.Warn().Err(err).Msg("Invalid message")
				continue
			}

			d.handleMessage(msg)
		}
	}
}

func (d *Daemon) handleMessage(msg protocol.Message) {
	switch msg.Type {
	case protocol.MsgConnect:
		d.handleConnectRequest(msg)
	case protocol.MsgAnswer:
		d.handleWebRTCMessage(msg, "answer", func(e *webrtc.Engine) error {
			d.log.Debug().Str("room", msg.Room).Msg("Forwarding answer to WebRTC engine")
			return e.HandleAnswer(msg)
		})
	case protocol.MsgICECandidate:
		d.handleWebRTCMessage(msg, "ice_candidate", func(e *webrtc.Engine) error {
			return e.HandleICE(msg)
		})
	case protocol.MsgOffer:
		// Browser might send renegotiation offers
		d.handleWebRTCMessage(msg, "offer", func(e *webrtc.Engine) error {
			return e.HandleOffer(msg)
		})
	case protocol.MsgPeerLeft:
		d.handlePeerDisconnect(msg)
	case protocol.MsgError:
		d.handleError(msg)
	}
}

// handleWebRTCMessage finds the session for the message's room and forwards
// the WebRTC signaling message to the session's engine.
func (d *Daemon) handleWebRTCMessage(msg protocol.Message, msgType string, handler func(*webrtc.Engine) error) {
	roomID := msg.Room
	if roomID == "" {
		d.log.Warn().Str("type", msgType).Msg("WebRTC message without room ID, dropping")
		return
	}
	session := d.getSession(roomID)
	if session == nil {
		d.log.Warn().Str("type", msgType).Str("room", roomID).Msg("No session for WebRTC message, dropping")
		return
	}
	if session.WebRTC == nil {
		d.log.Warn().Str("room", roomID).Msg("Session has no WebRTC engine")
		return
	}
	if err := handler(session.WebRTC); err != nil {
		d.log.Error().Err(err).Str("type", msgType).Str("room", roomID).Msg("WebRTC handler failed")
	}
}

func (d *Daemon) handleConnectRequest(msg protocol.Message) {
	var payload struct {
		Room     string `json:"room"`
		ClientID string `json:"client_id"`
	}
	json.Unmarshal(msg.Payload, &payload)

	d.log.Info().Str("client", payload.ClientID).Str("room", payload.Room).
		Msg("Incoming client connection")

	// Check max sessions
	d.mu.RLock()
	activeSessions := len(d.sessions)
	d.mu.RUnlock()
	if d.cfg.MaxSessions > 0 && activeSessions >= d.cfg.MaxSessions {
		d.log.Warn().
			Int("active", activeSessions).
			Int("max", d.cfg.MaxSessions).
			Msg("Max sessions reached, rejecting connection")
		d.sendError(payload.Room, 4001, fmt.Sprintf("Max sessions (%d) reached", d.cfg.MaxSessions))
		return
	}

	// Check allow list
	if len(d.cfg.AllowList) > 0 {
		allowed := false
		for _, id := range d.cfg.AllowList {
			if id == payload.ClientID || id == "*" {
				allowed = true
				break
			}
		}
		if !allowed {
			d.log.Warn().Str("client", payload.ClientID).Msg("Client not in allow list")
			return
		}
	}

	// Create WebRTC engine for this session
	engine, err := webrtc.NewEngine(func(cfg *webrtc.EngineConfig) {
		cfg.SignalConn = webrtc.NewSafeConn(d.signalConn)
		cfg.RoomID = payload.Room
		cfg.OnDataChannel = d.onDataChannel(payload.Room)
		cfg.ICEServers = []string{"stun:stun.l.google.com:19302"}
		cfg.Reconnect = webrtc.ReconnectConfig{
			InitialBackoff: 5 * time.Second,
			MaxBackoff:     60 * time.Second,
			MaxAttempts:    10,
			OnReconnectStart: func(attempt int) {
				d.log.Warn().
					Str("room", payload.Room).
					Int("attempt", attempt).
					Msg("WebRTC ICE restart attempt")
			},
			OnReconnectSuccess: func() {
				d.log.Info().
					Str("room", payload.Room).
					Msg("WebRTC ICE restart succeeded")
			},
			OnReconnectFailed: func() {
				d.log.Error().
					Str("room", payload.Room).
					Msg("WebRTC ICE restart failed, cleaning up session")
				d.cleanupSession(payload.Room)
			},
		}
	})
	if err != nil {
		d.log.Error().Err(err).Msg("Failed to create WebRTC engine")
		return
	}

	session := &Session{
		ID:        payload.Room,
		ClientID:  payload.ClientID,
		RoomID:    payload.Room,
		WebRTC:    engine,
		Authed:    d.cfg.MasterHash == "", // auto-auth when no password configured
		CreatedAt: time.Now(),
	}

	d.mu.Lock()
	d.sessions[payload.Room] = session
	d.mu.Unlock()

	// Create data channels BEFORE the offer so they're included in the SDP.
	// The browser/client receives them via OnDataChannel.
	engine.CreateDataChannel(webrtc.DataChannelAuth)
	engine.CreateDataChannel(webrtc.DataChannelTerminal)
	engine.CreateDataChannel(webrtc.DataChannelScreen)
	engine.CreateDataChannel(webrtc.DataChannelTransfer)
	engine.CreateDataChannel(webrtc.DataChannelClipboard)
	engine.CreateDataChannel(webrtc.DataChannelFile)
	d.log.Debug().Str("room", payload.Room).Msg("Data channels created on engine")

	// Create and send WebRTC offer
	offer, err := engine.CreateOffer()
	if err != nil {
		d.log.Error().Err(err).Msg("Failed to create WebRTC offer")
		return
	}

	offerMsg := protocol.NewMessage(protocol.MsgOffer, offer)
	offerMsg.Room = payload.Room
	d.signalConn.WriteJSON(offerMsg)
	d.log.Debug().Str("room", payload.Room).Msg("WebRTC offer sent")
}

func (d *Daemon) onDataChannel(roomID string) func(*webrtc.DataChannel, string) {
	return func(dc *webrtc.DataChannel, label string) {
		d.log.Info().Str("label", label).Str("room", roomID).
			Msg("Data channel opened")

		session := d.getSession(roomID)
		if session == nil {
			return
		}

		switch label {
		case "auth":
			d.handleAuthChannel(session, dc)
		case "terminal":
			d.handleTerminalChannel(session, dc)
		case "screen":
			d.handleScreenChannel(session, dc)
		case "transfer":
			d.handleTransferChannel(session, dc)
		case "file":
			d.handleFileChannel(session, dc)
		case "clipboard":
			d.handleClipboardChannel(session, dc)
		}
	}
}

func (d *Daemon) handleAuthChannel(session *Session, dc *webrtc.DataChannel) {
	dc.OnMessage(func(data []byte) {
		var msg protocol.Message
		if err := json.Unmarshal(data, &msg); err != nil {
			return
		}

		if msg.Type == protocol.MsgAuth {
			var authPayload protocol.AuthPayload
			json.Unmarshal(msg.Payload, &authPayload)

			valid := d.cfg.MasterHash == "" ||
				auth.CheckPassword(authPayload.Password, d.cfg.MasterHash)

			if valid {
				session.Authed = true
				dc.SendJSON(protocol.NewMessage(protocol.MsgAuthOK, nil))
				d.log.Info().Str("client", session.ClientID).Msg("Client authenticated")
			} else {
				dc.SendJSON(protocol.NewMessage(protocol.MsgAuthFail, nil))
				d.log.Warn().Str("client", session.ClientID).Msg("Authentication failed")
			}
		}
	})
}

func (d *Daemon) handleTerminalChannel(session *Session, dc *webrtc.DataChannel) {
	if !session.Authed {
		dc.SendJSON(protocol.NewMessage(protocol.MsgError, "Not authenticated"))
		return
	}

	// Spawn PTY
	shell, err := d.ptyMgr.Spawn(24, 80)
	if err != nil {
		d.log.Error().Err(err).Msg("Failed to spawn PTY")
		dc.SendJSON(protocol.NewMessage(protocol.MsgError, "Failed to start shell"))
		return
	}
	session.PTYSess = shell

	// Pipe PTY output → DataChannel
	go func() {
		buf := make([]byte, 32768)
		for {
			n, err := shell.Read(buf)
			if err != nil {
				break
			}
			if n > 0 {
				dc.Send(buf[:n])
			}
		}
	}()

	// Pipe DataChannel input → PTY
	dc.OnMessage(func(data []byte) {
		if !session.Authed {
			return
		}

		var msg protocol.Message
		if err := json.Unmarshal(data, &msg); err != nil {
			// Raw input (fast path)
			shell.Write(data)
			return
		}

		switch msg.Type {
		case protocol.MsgInput:
			var input string
			json.Unmarshal(msg.Payload, &input)
			shell.Write([]byte(input))
		case protocol.MsgResize:
			var resize protocol.ResizePayload
			json.Unmarshal(msg.Payload, &resize)
			shell.Resize(resize.Rows, resize.Cols)
		}
	})
}

func (d *Daemon) handleScreenChannel(session *Session, dc *webrtc.DataChannel) {
	if !session.Authed {
		dc.SendJSON(protocol.NewMessage(protocol.MsgError, "Not authenticated"))
		return
	}

	dc.OnMessage(func(data []byte) {
		var msg protocol.Message
		if err := json.Unmarshal(data, &msg); err != nil {
			return
		}

		switch msg.Type {
		case protocol.MsgScreenStart:
			// Stop any existing streamer first
			if session.ScreenStreamer != nil {
				session.ScreenStreamer.Stop()
				session.ScreenStreamer = nil
			}

			var cfg protocol.ScreenConfigPayload
			if msg.Payload != nil {
				json.Unmarshal(msg.Payload, &cfg)
			}

			streamCfg := screen.DefaultStreamConfig()
			if cfg.FPS > 0 {
				streamCfg.FPS = cfg.FPS
			}
			if cfg.Quality > 0 {
				streamCfg.Quality = cfg.Quality
			}
			if cfg.MaxDimension > 0 {
				streamCfg.MaxWidth = cfg.MaxDimension
				streamCfg.MaxHeight = cfg.MaxDimension
			}

			var streamer *screen.Streamer
			var err error
			streamer, err = screen.NewStreamer(streamCfg, func(frameData []byte, width, height int, _ time.Time, _ time.Duration) bool {
				// Only send if this streamer is still the active one
				if session.ScreenStreamer != streamer {
					return false
				}

				// Base64-encode JPEG data for JSON transport
				encoded := base64.StdEncoding.EncodeToString(frameData)
				framePayload := map[string]interface{}{
					"width":  width,
					"height": height,
					"data":   encoded,
				}
				frameMsg := protocol.NewMessage(protocol.MsgScreenFrame, framePayload)
				if err := dc.SendJSON(frameMsg); err != nil {
					d.log.Warn().Err(err).Msg("Failed to send screen frame")
					return false
				}
				return true
			})
			if err != nil {
				d.log.Error().Err(err).Msg("Failed to create screen streamer")
				dc.SendJSON(protocol.NewMessage(protocol.MsgError, "Failed to start screen capture"))
				return
			}

			session.ScreenStreamer = streamer
			if err := streamer.StartAsync(); err != nil {
				d.log.Error().Err(err).Msg("Failed to start screen streamer")
				session.ScreenStreamer = nil
				dc.SendJSON(protocol.NewMessage(protocol.MsgError, "Failed to start screen capture"))
				return
			}

			d.log.Info().Msg("Screen sharing started")

		case protocol.MsgScreenStop:
			if session.ScreenStreamer != nil {
				session.ScreenStreamer.Stop()
				session.ScreenStreamer = nil
				d.log.Info().Msg("Screen sharing stopped")
			}

		case protocol.MsgMouseMove:
			var payload protocol.MouseMovePayload
			if err := json.Unmarshal(msg.Payload, &payload); err != nil {
				return
			}
			if err := screen.MouseMove(payload.X, payload.Y); err != nil {
				d.log.Warn().Err(err).Msg("MouseMove failed")
			}

		case protocol.MsgMouseClick:
			var payload protocol.MouseClickPayload
			if err := json.Unmarshal(msg.Payload, &payload); err != nil {
				return
			}
			if payload.Down {
				if err := screen.MouseButtonDown(payload.Button, payload.X, payload.Y); err != nil {
					d.log.Warn().Err(err).Msg("MouseButtonDown failed")
				}
			} else {
				if err := screen.MouseButtonUp(payload.Button, payload.X, payload.Y); err != nil {
					d.log.Warn().Err(err).Msg("MouseButtonUp failed")
				}
			}

		case protocol.MsgMouseScroll:
			var payload protocol.MouseScrollPayload
			if err := json.Unmarshal(msg.Payload, &payload); err != nil {
				return
			}
			if err := screen.MouseScroll(payload.DeltaX, payload.DeltaY); err != nil {
				d.log.Warn().Err(err).Msg("MouseScroll failed")
			}

		case protocol.MsgKeyPress:
			var payload protocol.KeyPayload
			if err := json.Unmarshal(msg.Payload, &payload); err != nil {
				return
			}
			if payload.Chars != "" {
				// Send each character as a key press
				for _, ch := range payload.Chars {
					keyCode := screen.StringToKeyCode(string(ch))
					if keyCode != 0 {
						if err := screen.KeyPress(keyCode); err != nil {
							d.log.Warn().Err(err).Msg("KeyPress failed for char")
						}
					}
				}
			} else if payload.KeyCode != 0 {
				if err := screen.KeyPress(payload.KeyCode); err != nil {
					d.log.Warn().Err(err).Msg("KeyPress failed")
				}
			}

		case protocol.MsgKeyRelease:
			var payload protocol.KeyPayload
			if err := json.Unmarshal(msg.Payload, &payload); err != nil {
				return
			}
			var keyCode uint16
			if payload.Chars != "" {
				keyCode = screen.StringToKeyCode(payload.Chars)
			} else {
				keyCode = payload.KeyCode
			}
			if keyCode != 0 {
				if err := screen.KeyRelease(keyCode); err != nil {
					d.log.Warn().Err(err).Msg("KeyRelease failed")
				}
			}
		}
	})
}

func (d *Daemon) handleTransferChannel(session *Session, dc *webrtc.DataChannel) {
	if !session.Authed {
		dc.SendJSON(protocol.NewMessage(protocol.MsgError, "Not authenticated"))
		return
	}

	dc.OnMessage(func(data []byte) {
		var msg protocol.Message
		if err := json.Unmarshal(data, &msg); err != nil {
			return
		}

		switch msg.Type {
		case protocol.MsgFileRequest:
			var req protocol.FileRequestPayload
			if err := json.Unmarshal(msg.Payload, &req); err != nil {
				return
			}

			// Prepare to receive the file
			t, err := d.transferMgr.InitiateReceive(req)
			if err != nil {
				d.log.Error().Err(err).Msg("Failed to initiate receive")
				dc.SendJSON(protocol.NewMessage(protocol.MsgFileError,
					protocol.FileTransferErrorPayload{
						TransferID: req.TransferID,
						Code:       "init_failed",
						Message:    err.Error(),
					}))
				return
			}

			d.log.Info().
				Str("transfer_id", t.ID).
				Str("name", t.Name).
				Int64("size", t.Size).
				Msg("Incoming file transfer, accepting")

			// Auto-accept the file transfer
			dc.SendJSON(protocol.NewMessage(protocol.MsgFileAccept, map[string]interface{}{
				"transfer_id": req.TransferID,
			}))

		case protocol.MsgFileChunk:
			var chunk protocol.FileChunkPayload
			if err := json.Unmarshal(msg.Payload, &chunk); err != nil {
				return
			}

			t := d.transferMgr.Get(chunk.TransferID)
			if t == nil {
				d.log.Warn().Str("transfer_id", chunk.TransferID).Msg("Unknown transfer for chunk")
				return
			}

			if err := t.WriteChunk(chunk.Index, chunk.Data, chunk.Checksum); err != nil {
				d.log.Error().Err(err).
					Str("transfer_id", chunk.TransferID).
					Int("chunk", chunk.Index).
					Msg("Failed to write chunk")
				dc.SendJSON(protocol.NewMessage(protocol.MsgFileError,
					protocol.FileTransferErrorPayload{
						TransferID: chunk.TransferID,
						Code:       "write_failed",
						Message:    err.Error(),
					}))
				return
			}

			// Report progress
			dc.SendJSON(protocol.NewMessage(protocol.MsgFileProgress,
				protocol.FileProgressPayload{
					TransferID: chunk.TransferID,
					BytesSent:  t.BytesSent,
					TotalBytes: t.Size,
				}))

		case protocol.MsgFileComplete:
			var complete protocol.FileTransferCompletePayload
			if err := json.Unmarshal(msg.Payload, &complete); err != nil {
				return
			}

			t := d.transferMgr.Get(complete.TransferID)
			if t == nil {
				return
			}
			t.Complete()
			d.log.Info().
				Str("name", t.Name).
				Str("path", t.Path).
				Int64("size", complete.Size).
				Msg("File transfer completed")

		case protocol.MsgFileCancel:
			var cancelPayload struct {
				TransferID string `json:"transfer_id"`
			}
			if err := json.Unmarshal(msg.Payload, &cancelPayload); err != nil {
				return
			}
			t := d.transferMgr.Get(cancelPayload.TransferID)
			if t != nil {
				t.Cancel()
				d.log.Info().Str("transfer_id", cancelPayload.TransferID).Msg("File transfer cancelled")
			}

		case protocol.MsgFileError:
			var errPayload protocol.FileTransferErrorPayload
			if err := json.Unmarshal(msg.Payload, &errPayload); err != nil {
				return
			}
			t := d.transferMgr.Get(errPayload.TransferID)
			if t != nil {
				t.Cancel()
			}
			d.log.Warn().
				Str("transfer_id", errPayload.TransferID).
				Str("code", errPayload.Code).
				Str("message", errPayload.Message).
				Msg("File transfer error from client")
		}
	})
}

func (d *Daemon) handleFileChannel(session *Session, dc *webrtc.DataChannel) {
	if !session.Authed {
		dc.SendJSON(protocol.NewMessage(protocol.MsgError, "Not authenticated"))
		return
	}

	d.log.Info().Str("label", "file").Msg("File channel handler initialized")

	dc.OnMessage(func(data []byte) {
		var msg protocol.Message
		if err := json.Unmarshal(data, &msg); err != nil {
			d.log.Warn().Err(err).Msg("Failed to unmarshal file channel message")
			return
		}

		switch msg.Type {
		case protocol.MsgFileRequest:
			var req protocol.FileRequestPayload
			if err := json.Unmarshal(msg.Payload, &req); err != nil {
				d.log.Warn().Err(err).Msg("Failed to unmarshal file request")
				return
			}

			// Initiate receive via transfer manager
			t, err := d.transferMgr.InitiateReceive(req)
			if err != nil {
				d.log.Error().Err(err).Msg("Failed to initiate file receive")
				dc.SendJSON(protocol.NewMessage(protocol.MsgFileError,
					protocol.FileTransferErrorPayload{
						TransferID: req.TransferID,
						Code:       "init_failed",
						Message:    err.Error(),
					}))
				return
			}

			d.log.Info().
				Str("transfer_id", t.ID).
				Str("name", t.Name).
				Int64("size", t.Size).
				Msg("Incoming file transfer, auto-accepting")

			// Auto-accept the transfer
			dc.SendJSON(protocol.NewMessage(protocol.MsgFileAccept, map[string]interface{}{
				"transfer_id": req.TransferID,
			}))

		case protocol.MsgFileChunk:
			var chunk protocol.FileChunkPayload
			if err := json.Unmarshal(msg.Payload, &chunk); err != nil {
				d.log.Warn().Err(err).Msg("Failed to unmarshal file chunk")
				return
			}

			// Decode base64 data if needed (Data is []byte, JSON auto-decodes,
			// but some clients may send base64 string that needs explicit handling)
			if len(chunk.Data) > 0 {
				// Check if the data looks like a base64 string (ASCII printable range)
				decoded, err := base64.StdEncoding.DecodeString(string(chunk.Data))
				if err == nil && len(decoded) > 0 {
					chunk.Data = decoded
				}
			}

			t := d.transferMgr.Get(chunk.TransferID)
			if t == nil {
				d.log.Warn().Str("transfer_id", chunk.TransferID).Msg("Unknown transfer for chunk")
				dc.SendJSON(protocol.NewMessage(protocol.MsgFileError,
					protocol.FileTransferErrorPayload{
						TransferID: chunk.TransferID,
						Code:       "unknown_transfer",
						Message:    "No active transfer with this ID",
					}))
				return
			}

			if err := t.WriteChunk(chunk.Index, chunk.Data, chunk.Checksum); err != nil {
				d.log.Error().Err(err).
					Str("transfer_id", chunk.TransferID).
					Int("chunk", chunk.Index).
					Msg("Failed to write chunk")
				dc.SendJSON(protocol.NewMessage(protocol.MsgFileError,
					protocol.FileTransferErrorPayload{
						TransferID: chunk.TransferID,
						Code:       "write_failed",
						Message:    err.Error(),
					}))
				return
			}

			// Report progress
			dc.SendJSON(protocol.NewMessage(protocol.MsgFileProgress,
				protocol.FileProgressPayload{
					TransferID: chunk.TransferID,
					BytesSent:  t.BytesSent,
					TotalBytes: t.Size,
				}))

		case protocol.MsgFileComplete:
			var complete protocol.FileTransferCompletePayload
			if err := json.Unmarshal(msg.Payload, &complete); err != nil {
				d.log.Warn().Err(err).Msg("Failed to unmarshal file complete")
				return
			}

			t := d.transferMgr.Get(complete.TransferID)
			if t == nil {
				d.log.Warn().Str("transfer_id", complete.TransferID).Msg("Unknown transfer for completion")
				return
			}
			t.Complete()

			d.log.Info().
				Str("name", t.Name).
				Str("path", t.Path).
				Int64("size", complete.Size).
				Msg("File transfer completed")

		case protocol.MsgFileCancel:
			var cancelPayload struct {
				TransferID string `json:"transfer_id"`
			}
			if err := json.Unmarshal(msg.Payload, &cancelPayload); err != nil {
				d.log.Warn().Err(err).Msg("Failed to unmarshal file cancel")
				return
			}
			t := d.transferMgr.Get(cancelPayload.TransferID)
			if t != nil {
				t.Cancel()
				d.log.Info().
					Str("transfer_id", cancelPayload.TransferID).
					Msg("File transfer cancelled by peer")
			}

		case protocol.MsgFileError:
			var errPayload protocol.FileTransferErrorPayload
			if err := json.Unmarshal(msg.Payload, &errPayload); err != nil {
				d.log.Warn().Err(err).Msg("Failed to unmarshal file error")
				return
			}
			t := d.transferMgr.Get(errPayload.TransferID)
			if t != nil {
				t.Cancel()
			}
			d.log.Warn().
				Str("transfer_id", errPayload.TransferID).
				Str("code", errPayload.Code).
				Str("message", errPayload.Message).
				Msg("File transfer error from peer")
		}
	})
}

func (d *Daemon) handleClipboardChannel(session *Session, dc *webrtc.DataChannel) {
	// Start clipboard monitoring when the clipboard channel opens
	if d.clipMon == nil {
		d.clipMon = NewClipboardMonitor(d.log)
	}

	// Start the monitor
	if err := d.clipMon.Start(); err != nil {
		d.log.Warn().Err(err).Msg("Failed to start clipboard monitor")
		return
	}

	// Register callback for host-to-client clipboard changes
	d.clipMon.OnChange(func(text string) {
		d.sendClipboardUpdate(dc, text)
	})

	// Send initial clipboard content to client
	if content, err := d.clipMon.Get(); err == nil && content != "" {
		d.sendClipboardUpdate(dc, content)
	}

	// Handle incoming messages from client
	dc.OnMessage(func(data []byte) {
		var msg protocol.Message
		if err := json.Unmarshal(data, &msg); err != nil {
			return
		}

		switch msg.Type {
		case protocol.MsgClipboardData:
			d.handleClipboardData(dc, msg)
		case protocol.MsgClipboardRequest:
			// Respond with current clipboard content
			if content, err := d.clipMon.Get(); err == nil && content != "" {
				d.sendClipboardUpdate(dc, content)
			}
		}
	})

	d.log.Info().Msg("Clipboard channel initialized")
}

func (d *Daemon) handlePeerDisconnect(msg protocol.Message) {
	var payload struct {
		PeerID string `json:"peer_id"`
	}
	json.Unmarshal(msg.Payload, &payload)

	d.mu.Lock()
	for id, sess := range d.sessions {
		if sess.ClientID == payload.PeerID {
			if sess.WebRTC != nil {
				sess.WebRTC.Close()
			}
			d.cleanupSessionLocked(id, sess)
		}
	}
	d.mu.Unlock()

	d.log.Info().Str("peer", payload.PeerID).Msg("Client disconnected")
}

// cleanupSession removes and cleans up a single session by room ID.
func (d *Daemon) cleanupSession(roomID string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	sess, ok := d.sessions[roomID]
	if !ok {
		return
	}
	d.cleanupSessionLocked(roomID, sess)
}

// cleanupSessionLocked removes and cleans up a session (caller must hold d.mu).
func (d *Daemon) cleanupSessionLocked(roomID string, sess *Session) {
	if sess.PTYSess != nil {
		sess.PTYSess.Close()
	}
	if sess.ScreenStreamer != nil {
		sess.ScreenStreamer.Stop()
	}
	if sess.WebRTC != nil {
		sess.WebRTC.Close()
	}
	delete(d.sessions, roomID)
	d.log.Info().Str("room", roomID).Msg("Session cleaned up")
}

func (d *Daemon) handleError(msg protocol.Message) {
	var err protocol.ErrorPayload
	json.Unmarshal(msg.Payload, &err)
	d.log.Warn().Str("error", err.Message).Msg("Received error from signal server")
}

func (d *Daemon) getSession(roomID string) *Session {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.sessions[roomID]
}

// sendError sends an error message back to the signal server.
func (d *Daemon) sendError(roomID string, code int, message string) {
	if d.signalConn != nil {
		d.signalConn.WriteJSON(protocol.NewMessage(protocol.MsgError, protocol.ErrorPayload{
			Code:    code,
			Message: message,
		}))
	}
}

// APIResponse is the JSON envelope for local API responses.
type APIResponse struct {
	Success  bool           `json:"success"`
	Sessions []APISession   `json:"sessions,omitempty"`
	Error    string         `json:"error,omitempty"`
}

// APISession is the public session representation for the local API.
type APISession struct {
	ID        string `json:"id"`
	ClientID  string `json:"client_id"`
	CreatedAt string `json:"created_at"`
	Duration  string `json:"duration"`
	Authed    bool   `json:"authed"`
}

// startLocalAPI starts a local HTTP server on 127.0.0.1:9876 for the macOS menu bar app.
func (d *Daemon) startLocalAPI() {
	d.localAPIOnce.Do(func() {
		mux := http.NewServeMux()
		mux.HandleFunc("/api/sessions", d.handleSessionsAPI)

		d.localAPI = &http.Server{
			Addr:    "127.0.0.1:9876",
			Handler: mux,
		}

		listener, err := net.Listen("tcp", "127.0.0.1:9876")
		if err != nil {
			d.log.Warn().Err(err).Msg("Failed to start local API server (port 9876 may be in use)")
			return
		}

		go func() {
			d.log.Info().Msg("Local API server listening on 127.0.0.1:9876")
			if err := d.localAPI.Serve(listener); err != nil && err != http.ErrServerClosed {
				d.log.Warn().Err(err).Msg("Local API server stopped")
			}
		}()
	})
}

func (d *Daemon) handleSessionsAPI(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	d.mu.RLock()
	apiSessions := make([]APISession, 0, len(d.sessions))
	for _, sess := range d.sessions {
		dur := time.Since(sess.CreatedAt)
		apiSessions = append(apiSessions, APISession{
			ID:        sess.ID,
			ClientID:  sess.ClientID,
			CreatedAt: sess.CreatedAt.Format(time.RFC3339),
			Duration:  fmt.Sprintf("%dm%ds", int(dur.Minutes()), int(dur.Seconds())%60),
			Authed:    sess.Authed,
		})
	}
	d.mu.RUnlock()

	resp := APIResponse{Success: true, Sessions: apiSessions}
	json.NewEncoder(w).Encode(resp)
}

func (d *Daemon) cleanup() {
	d.log.Info().Msg("Cleaning up host daemon")

	// Stop clipboard monitor
	if d.clipMon != nil {
		d.clipMon.Stop()
	}

	// Shutdown local API server
	if d.localAPI != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		if err := d.localAPI.Shutdown(ctx); err != nil {
			d.log.Warn().Err(err).Msg("Local API shutdown error")
		}
	}

	d.mu.Lock()
	defer d.mu.Unlock()

	for id, sess := range d.sessions {
		d.cleanupSessionLocked(id, sess)
	}

	if d.signalConn != nil {
		d.signalConn.Close()
	}
}
