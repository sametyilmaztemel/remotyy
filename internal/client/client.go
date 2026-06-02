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