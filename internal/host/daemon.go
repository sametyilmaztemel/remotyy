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