// Package client provides the remotty client for connecting to remote hosts.
package client

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	gosignal "os/signal"
	"sync"
	"syscall"

	"github.com/gorilla/websocket"
	"github.com/rs/zerolog"
	"github.com/sametyilmaztemel/remotty/internal/config"
	"github.com/sametyilmaztemel/remotty/internal/protocol"
	"github.com/sametyilmaztemel/remotty/internal/webrtc"
	"golang.org/x/term"
)

// Client connects to the signaling server and establishes WebRTC sessions.
type Client struct {
	cfg       config.ClientConfig
	signalConn *websocket.Conn
	webrtcEng *webrtc.Engine
	hosts     []protocol.HostInfo
	log       zerolog.Logger
}

// NewClient creates a new client.
func NewClient(cfg config.ClientConfig, log zerolog.Logger) (*Client, error) {
	return &Client{
		cfg: cfg,
		log: log.With().Str("component", "client").Logger(),
	}, nil
}

// ListHosts fetches the list of available hosts.
func (c *Client) ListHosts() ([]protocol.HostInfo, error) {
	conn, _, err := websocket.DefaultDialer.Dial(c.cfg.SignalURL+"/ws", nil)
	if err != nil {
		return nil, fmt.Errorf("connect: %w", err)
	}
	defer conn.Close()

	conn.WriteJSON(protocol.NewMessage(protocol.MsgListHosts, nil))

	_, data, err := conn.ReadMessage()
	if err != nil {
		return nil, fmt.Errorf("read: %w", err)
	}

	var msg protocol.Message
	json.Unmarshal(data, &msg)

	if msg.Type == protocol.MsgHostList {
		var payload struct {
			Hosts []protocol.HostInfo `json:"hosts"`
		}
		json.Unmarshal(msg.Payload, &payload)
		c.hosts = payload.Hosts
		return payload.Hosts, nil
	}

	return nil, fmt.Errorf("unexpected response: %s", msg.Type)
}

// ConnectInteractive connects to a host and starts an interactive terminal.
func (c *Client) ConnectInteractive(ctx context.Context) error {
	conn, _, err := websocket.DefaultDialer.DialContext(ctx, c.cfg.SignalURL+"/ws", nil)
	if err != nil {
		return fmt.Errorf("connect: %w", err)
	}
	c.signalConn = conn
	defer conn.Close()

	// Request connection to specific host
	conn.WriteJSON(protocol.NewMessage(protocol.MsgConnect, protocol.ConnectPayload{
		HostID:   c.cfg.HostID,
		Password: c.cfg.MasterPassword,
	}))

	// Wait for room ready
	_, data, err := conn.ReadMessage()
	if err != nil {
		return err
	}

	var msg protocol.Message
	json.Unmarshal(data, &msg)

	if msg.Type == protocol.MsgError {
		var errPayload protocol.ErrorPayload
		json.Unmarshal(msg.Payload, &errPayload)
		return fmt.Errorf("connection rejected: %s", errPayload.Message)
	}

	if msg.Type != protocol.MsgRoomReady {
		return fmt.Errorf("unexpected message: %s", msg.Type)
	}

	c.log.Info().Msg("Room ready, starting WebRTC negotiation")

	// Capture room info
	var roomInfo struct {
		Room   string            `json:"room"`
		HostID string            `json:"host_id"`
		Host   protocol.HostInfo `json:"host"`
	}
	json.Unmarshal(msg.Payload, &roomInfo)

	// Create WebRTC engine
	engine, err := webrtc.NewEngine(func(cfg *webrtc.EngineConfig) {
		cfg.SignalConn = webrtc.NewSafeConn(conn)
		cfg.RoomID = roomInfo.Room
		cfg.ICEServers = []string{"stun:stun.l.google.com:19302"}
	})
	if err != nil {
		return fmt.Errorf("create webrtc engine: %w", err)
	}
	c.webrtcEng = engine

	// Read signaling messages in background
	go func() {
		for {
			_, data, err := conn.ReadMessage()
			if err != nil {
				return
			}
			var sigMsg protocol.Message
			json.Unmarshal(data, &sigMsg)

			switch sigMsg.Type {
			case protocol.MsgOffer:
				engine.HandleOffer(sigMsg)
			case protocol.MsgICECandidate:
				engine.HandleICE(sigMsg)
			}
		}
	}()

	// Wait for offer from host
	// The host sends the offer after we request connection
	// For now, the host initiates the offer on MsgConnect

	fmt.Println("\n🔗 Connected! Starting interactive terminal...")
	fmt.Println("   NOTE: Run in a real terminal for full TTY support.")
	fmt.Println("Press Ctrl+Q to disconnect.")

	// Terminal setup
	oldState, err := term.MakeRaw(0)
	if err != nil {
		return fmt.Errorf("make raw terminal: %w", err)
	}
	defer term.Restore(0, oldState)

	// Get terminal size
	width, height, err := term.GetSize(0)
	if err != nil {
		width, height = 80, 24
	}

	terminalDC := engine.CreateDataChannel("terminal")
	authDC := engine.CreateDataChannel("auth")
	transferDC := engine.CreateDataChannel("transfer")

	// Send auth if password provided
	if c.cfg.MasterPassword != "" {
		authDC.SendJSON(protocol.NewMessage(protocol.MsgAuth, protocol.AuthPayload{
			Password: c.cfg.MasterPassword,
		}))
	}

	// Send initial resize
	terminalDC.SendJSON(protocol.NewMessage(protocol.MsgResize, protocol.ResizePayload{
		Rows: uint16(height),
		Cols: uint16(width),
	}))

	// Read from terminal and send to WebRTC
	go func() {
		buf := make([]byte, 4096)
		for {
			n, err := os.Stdin.Read(buf)
			if err != nil {
				return
			}
			if n > 0 {
				terminalDC.Send(buf[:n])
			}
		}
	}()

	// Monitor terminal resize (SIGWINCH) and send MsgResize events
	var lastRows, lastCols int
	lastRows, lastCols = height, width
	resizeCh := make(chan os.Signal, 1)
	gosignal.Notify(resizeCh, syscall.SIGWINCH)
	defer gosignal.Stop(resizeCh)

	var resizeMu sync.Mutex
	go func() {
		for range resizeCh {
			w, h, err := term.GetSize(0)
			if err != nil {
				continue
			}
			resizeMu.Lock()
			if w == lastCols && h == lastRows {
				resizeMu.Unlock()
				continue
			}
			lastCols, lastRows = w, h
			resizeMu.Unlock()

			c.log.Debug().
				Int("rows", h).Int("cols", w).
				Msg("Terminal resized, sending resize event")
			terminalDC.SendJSON(protocol.NewMessage(protocol.MsgResize, protocol.ResizePayload{
				Rows: uint16(h),
				Cols: uint16(w),
			}))
		}
	}()

	// Write received data to stdout
	done := make(chan struct{})

	terminalDC.OnMessage(func(data []byte) {
		os.Stdout.Write(data)
	})

	authDC.OnMessage(func(data []byte) {
		var msg protocol.Message
		json.Unmarshal(data, &msg)
		if msg.Type == protocol.MsgAuthOK {
			fmt.Println("\r✅ Authenticated successfully")
		} else if msg.Type == protocol.MsgAuthFail {
			fmt.Println("\r❌ Authentication failed")
			close(done)
		}
	})

	// Handle transfer data channel messages (file transfer progress/errors)
	transferDC.OnMessage(func(data []byte) {
		var msg protocol.Message
		if err := json.Unmarshal(data, &msg); err != nil {
			return
		}

		switch msg.Type {
		case protocol.MsgFileProgress:
			var progress protocol.FileProgressPayload
			if err := json.Unmarshal(msg.Payload, &progress); err != nil {
				return
			}
			fmt.Printf("\r📦 Transfer %s: %d/%d bytes",
				progress.TransferID, progress.BytesSent, progress.TotalBytes)

		case protocol.MsgFileComplete:
			var complete protocol.FileTransferCompletePayload
			if err := json.Unmarshal(msg.Payload, &complete); err != nil {
				return
			}
			fmt.Printf("\r✅ Transfer %s complete: %d bytes\n",
				complete.TransferID, complete.Size)

		case protocol.MsgFileError:
			var errPayload protocol.FileTransferErrorPayload
			if err := json.Unmarshal(msg.Payload, &errPayload); err != nil {
				return
			}
			fmt.Printf("\r❌ Transfer %s error [%s]: %s\n",
				errPayload.TransferID, errPayload.Code, errPayload.Message)

		case protocol.MsgFileAccept:
			var acceptPayload struct {
				TransferID string `json:"transfer_id"`
			}
			if err := json.Unmarshal(msg.Payload, &acceptPayload); err != nil {
				return
			}
			fmt.Printf("\r✅ Transfer %s accepted by host\n", acceptPayload.TransferID)
		}
	})

	<-done
	return nil
}
