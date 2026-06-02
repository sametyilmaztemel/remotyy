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
