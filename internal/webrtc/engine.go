// Package webrtc provides a high-level WebRTC engine for remotty.
package webrtc

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/pion/interceptor"
	pion "github.com/pion/webrtc/v4"
	"github.com/rs/zerolog/log"
	"github.com/sametyilmaztemel/remotty/internal/protocol"
)


// SafeConn wraps *websocket.Conn with a mutex for concurrent-safe writes.
type SafeConn struct {
	conn *websocket.Conn
	mu   sync.Mutex
}

// NewSafeConn wraps a WebSocket connection for safe concurrent use.
func NewSafeConn(conn *websocket.Conn) *SafeConn {
	return &SafeConn{conn: conn}
}

// WriteJSON sends a JSON message, serialising concurrent calls.
func (s *SafeConn) WriteJSON(v interface{}) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.conn.WriteJSON(v)
}

// ReadJSON delegates to the underlying connection (no lock needed for reads).
func (s *SafeConn) ReadJSON(v interface{}) error {
	return s.conn.ReadJSON(v)
}

// Underlying returns the raw *websocket.Conn (use with care).
func (s *SafeConn) Underlying() *websocket.Conn { return s.conn }


// Data channel labels used across the system.
const (
	DataChannelAuth      = "auth"
	DataChannelTerminal  = "terminal"
	DataChannelScreen    = "screen"
	DataChannelTransfer  = "transfer"
	DataChannelClipboard = "clipboard"
	DataChannelFile      = "file"
)


// ReconnectConfig configures ICE restart reconnection behaviour.
type ReconnectConfig struct {
	// InitialBackoff is the initial delay before the first reconnection attempt.
	InitialBackoff time.Duration

	// MaxBackoff is the maximum delay between reconnection attempts.
	MaxBackoff time.Duration

	// MaxAttempts is the maximum number of reconnection attempts (0 = unlimited).
	MaxAttempts int

	// OnReconnectStart is called when a reconnection cycle begins.
	OnReconnectStart func(attempt int)

	// OnReconnectSuccess is called when reconnection succeeds.
	OnReconnectSuccess func()

	// OnReconnectFailed is called when all reconnection attempts are exhausted.
	OnReconnectFailed func()
}

// DefaultReconnectConfig returns sensible defaults for ICE restart reconnection.
func DefaultReconnectConfig() ReconnectConfig {
	return ReconnectConfig{
		InitialBackoff: 5 * time.Second,
		MaxBackoff:     60 * time.Second,
		MaxAttempts:    10,
	}
}

// Engine manages a WebRTC peer connection with data channels.
type Engine struct {
	pc             *pion.PeerConnection
	config         EngineConfig
	mu             sync.Mutex
	dataChannels   map[string]*pion.DataChannel
	closed         bool
	reconnectCfg   ReconnectConfig
	reconnectStop  chan struct{}
	restarting     bool
	restartMu      sync.Mutex
}

// EngineConfig for WebRTC setup.
type EngineConfig struct {
	SignalConn     *SafeConn
	RoomID         string
	ICEServers     []string
	OnDataChannel  func(dc *DataChannel, label string)
	OnICEState     func(state pion.ICEConnectionState)
	Reconnect      ReconnectConfig
}

// DataChannel wraps pion's DataChannel with convenience methods.
type DataChannel struct {
	*pion.DataChannel
	mu        sync.Mutex
	onMessage func([]byte)
}

// SendJSON sends a protocol.Message as JSON over the data channel.
func (dc *DataChannel) SendJSON(msg protocol.Message) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	return dc.Send(data)
}

// Send sends raw bytes.
func (dc *DataChannel) Send(data []byte) error {
	dc.mu.Lock()
	defer dc.mu.Unlock()
	return dc.DataChannel.Send(data)
}

// OnMessage registers a callback for data channel messages.
// The callback receives raw bytes.
func (dc *DataChannel) OnMessage(fn func([]byte)) {
	dc.onMessage = fn
	dc.DataChannel.OnMessage(func(msg pion.DataChannelMessage) {
		if fn != nil {
			fn(msg.Data)
		}
	})
}

// NewEngine creates a WebRTC engine with the given configuration.
func NewEngine(fn func(*EngineConfig)) (*Engine, error) {
	cfg := &EngineConfig{
		ICEServers: []string{"stun:stun.l.google.com:19302"},
		Reconnect:  DefaultReconnectConfig(),
	}
	fn(cfg)

	// Apply reconnect defaults for zero-value fields
	if cfg.Reconnect.InitialBackoff == 0 {
		cfg.Reconnect.InitialBackoff = 5 * time.Second
	}
	if cfg.Reconnect.MaxBackoff == 0 {
		cfg.Reconnect.MaxBackoff = 60 * time.Second
	}

	// ICE servers
	iceServers := make([]pion.ICEServer, len(cfg.ICEServers))
	for i, s := range cfg.ICEServers {
		iceServers[i] = pion.ICEServer{URLs: []string{s}}
	}

	// Create media engine with interceptor
	m := &pion.MediaEngine{}
	if err := m.RegisterDefaultCodecs(); err != nil {
		return nil, fmt.Errorf("register codecs: %w", err)
	}

	i := &interceptor.Registry{}
	if err := pion.RegisterDefaultInterceptors(m, i); err != nil {
		return nil, fmt.Errorf("register interceptors: %w", err)
	}

	api := pion.NewAPI(
		pion.WithMediaEngine(m),
		pion.WithInterceptorRegistry(i),
	)

	config := pion.Configuration{
		ICEServers:   iceServers,
		ICETransportPolicy: pion.ICETransportPolicyAll,
	}

	pc, err := api.NewPeerConnection(config)
	if err != nil {
		return nil, fmt.Errorf("create peer connection: %w", err)
	}

	e := &Engine{
		pc:            pc,
		config:        *cfg,
		dataChannels:  make(map[string]*pion.DataChannel),
		reconnectCfg:  cfg.Reconnect,
		reconnectStop: make(chan struct{}),
	}

	// ICE state handler
	pc.OnICEConnectionStateChange(func(state pion.ICEConnectionState) {
		log.Debug().Str("state", state.String()).Msg("ICE state changed")
		if cfg.OnICEState != nil {
			cfg.OnICEState(state)
		}

		switch state {
		case pion.ICEConnectionStateFailed,
			pion.ICEConnectionStateDisconnected:

			// Do NOT immediately close — attempt ICE restart with backoff
			go e.reconnectLoop()

		case pion.ICEConnectionStateConnected,
			pion.ICEConnectionStateCompleted:

			// Reconnection succeeded — stop any active reconnect cycle
			e.restartMu.Lock()
			if e.restarting {
				e.restarting = false
				select {
				case e.reconnectStop <- struct{}{}:
				default:
				}
				if e.reconnectCfg.OnReconnectSuccess != nil {
					e.reconnectCfg.OnReconnectSuccess()
				}
			}
			e.restartMu.Unlock()
		}
	})

	// Data channel handler
	pc.OnDataChannel(func(dc *pion.DataChannel) {
		e.mu.Lock()
		e.dataChannels[dc.Label()] = dc
		e.mu.Unlock()

		wrapped := &DataChannel{DataChannel: dc}

		dc.OnOpen(func() {
			log.Debug().Str("label", dc.Label()).Msg("Data channel opened")
			if cfg.OnDataChannel != nil {
				cfg.OnDataChannel(wrapped, dc.Label())
			}
		})

		dc.OnClose(func() {
			log.Debug().Str("label", dc.Label()).Msg("Data channel closed")
		})
	})

	// ICE candidate handler
	pc.OnICECandidate(func(candidate *pion.ICECandidate) {
		if candidate == nil || e.closed {
			return
		}
		msg := protocol.NewMessage(protocol.MsgICECandidate, candidate.ToJSON())
		msg.Room = e.config.RoomID
		if err := cfg.SignalConn.WriteJSON(msg); err != nil {
			log.Warn().Err(err).Msg("Failed to send ICE candidate")
		}
	})

	return e, nil
}

// CreateOffer creates and sets a local SDP offer.
func (e *Engine) CreateOffer() (map[string]interface{}, error) {
	offer, err := e.pc.CreateOffer(nil)
	if err != nil {
		return nil, fmt.Errorf("create offer: %w", err)
	}

	if err := e.pc.SetLocalDescription(offer); err != nil {
		return nil, fmt.Errorf("set local description: %w", err)
	}

	log.Debug().Msg("Created WebRTC offer")
	return map[string]interface{}{
		"type": offer.Type.String(),
		"sdp":  offer.SDP,
	}, nil
}

// HandleOffer processes a remote offer and sends an answer.
func (e *Engine) HandleOffer(msg protocol.Message) error {
	var offer struct {
		Type string `json:"type"`
		SDP  string `json:"sdp"`
	}
	if err := json.Unmarshal(msg.Payload, &offer); err != nil {
		return fmt.Errorf("parse offer: %w", err)
	}

	if err := e.pc.SetRemoteDescription(pion.SessionDescription{
		Type: pion.SDPTypeOffer,
		SDP:  offer.SDP,
	}); err != nil {
		return fmt.Errorf("set remote description: %w", err)
	}

	answer, err := e.pc.CreateAnswer(nil)
	if err != nil {
		return fmt.Errorf("create answer: %w", err)
	}

	if err := e.pc.SetLocalDescription(answer); err != nil {
		return fmt.Errorf("set local description: %w", err)
	}

	answerMsg := protocol.NewMessage(protocol.MsgAnswer, map[string]interface{}{
		"type": answer.Type.String(),
		"sdp":  answer.SDP,
	})
	answerMsg.Room = e.config.RoomID

	log.Debug().Msg("Created WebRTC answer")
	return e.config.SignalConn.WriteJSON(answerMsg)
}

// HandleAnswer processes a remote SDP answer.
func (e *Engine) HandleAnswer(msg protocol.Message) error {
	var answer struct {
		Type string `json:"type"`
		SDP  string `json:"sdp"`
	}
	if err := json.Unmarshal(msg.Payload, &answer); err != nil {
		return fmt.Errorf("parse answer: %w", err)
	}

	return e.pc.SetRemoteDescription(pion.SessionDescription{
		Type: pion.SDPTypeAnswer,
		SDP:  answer.SDP,
	})
}

// HandleICE processes an ICE candidate.
func (e *Engine) HandleICE(msg protocol.Message) error {
	var candidate pion.ICECandidateInit
	if err := json.Unmarshal(msg.Payload, &candidate); err != nil {
		return err
	}
	return e.pc.AddICECandidate(candidate)
}

// CreateDataChannel creates a new data channel and wires up the OnOpen
// callback so locally-created channels also trigger cfg.OnDataChannel
// (the same callback used for remote-received channels via OnDataChannel).
func (e *Engine) CreateDataChannel(label string) *DataChannel {
	dc, err := e.pc.CreateDataChannel(label, &pion.DataChannelInit{
		Ordered:        boolPtr(true),
		MaxRetransmits: nil, // retry until success
	})
	if err != nil {
		log.Error().Err(err).Str("label", label).Msg("Failed to create data channel")
		return nil
	}

	e.mu.Lock()
	e.dataChannels[label] = dc
	e.mu.Unlock()

	wrapped := &DataChannel{DataChannel: dc}

	// Register OnOpen to trigger cfg.OnDataChannel, matching the behaviour
	// of remote channels received via pc.OnDataChannel.
	dc.OnOpen(func() {
		log.Debug().Str("label", label).Msg("Locally-created data channel opened")
		if e.config.OnDataChannel != nil {
			e.config.OnDataChannel(wrapped, label)
		}
	})

	dc.OnClose(func() {
		log.Debug().Str("label", label).Msg("Data channel closed")
	})

	return wrapped
}

// AddVideoTrack adds a video track for screen sharing.
func (e *Engine) AddVideoTrack() (*pion.TrackLocalStaticSample, error) {
	track, err := pion.NewTrackLocalStaticSample(
		pion.RTPCodecCapability{MimeType: pion.MimeTypeVP8},
		"screen",
		"screen-id",
	)
	if err != nil {
		return nil, fmt.Errorf("create video track: %w", err)
	}

	_, err = e.pc.AddTrack(track)
	if err != nil {
		return nil, fmt.Errorf("add video track: %w", err)
	}

	log.Debug().Msg("Video track added for screen sharing")
	return track, nil
}

// reconnectLoop attempts ICE restart with exponential backoff.
// It runs in a goroutine and exits when reconnection succeeds, the engine is closed,
// or max attempts are exhausted.
func (e *Engine) reconnectLoop() {
	e.restartMu.Lock()
	if e.restarting {
		e.restartMu.Unlock()
		return // already reconnecting
	}
	e.restarting = true
	e.restartMu.Unlock()

	backoff := e.reconnectCfg.InitialBackoff
	attempt := 0
	max := e.reconnectCfg.MaxAttempts

	for {
		attempt++

		if e.reconnectCfg.OnReconnectStart != nil {
			e.reconnectCfg.OnReconnectStart(attempt)
		}

		log.Info().
			Int("attempt", attempt).
			Dur("backoff", backoff).
			Msg("ICE restart: reconnecting")

		// Perform ICE restart by creating a new offer with ICERestart flag
		if err := e.restartICE(); err != nil {
			log.Warn().
				Int("attempt", attempt).
				Err(err).
				Msg("ICE restart attempt failed")

			// Check if we've exhausted max attempts
			if max > 0 && attempt >= max {
				log.Error().
					Int("attempts", attempt).
					Msg("ICE restart max attempts exhausted, giving up")

				e.restartMu.Lock()
				e.restarting = false
				e.restartMu.Unlock()

				if e.reconnectCfg.OnReconnectFailed != nil {
					e.reconnectCfg.OnReconnectFailed()
				}
				e.Close()
				return
			}

			// Exponential backoff
			select {
			case <-e.reconnectStop:
				// We were told to stop (connection recovered or engine closed)
				e.restartMu.Lock()
				e.restarting = false
				e.restartMu.Unlock()
				return
			case <-time.After(backoff):
				backoff *= 2
				if backoff > e.reconnectCfg.MaxBackoff {
					backoff = e.reconnectCfg.MaxBackoff
				}
			}
			continue
		}

		// restartICE succeeded — connection recovered
		e.restartMu.Lock()
		e.restarting = false
		e.restartMu.Unlock()
		return
	}
}

// restartICE performs an ICE restart by creating a new offer.
// It sets the ICERestart flag to force new ICE candidates.
func (e *Engine) restartICE() error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.closed {
		return fmt.Errorf("engine is closed")
	}

	// Create a new offer with ICE restart
	offer, err := e.pc.CreateOffer(&pion.OfferOptions{
		ICERestart: true,
	})
	if err != nil {
		return fmt.Errorf("create ICE restart offer: %w", err)
	}

	if err := e.pc.SetLocalDescription(offer); err != nil {
		return fmt.Errorf("set local description for ICE restart: %w", err)
	}

	// Send the new offer via the signaling server
	offerMsg := protocol.NewMessage(protocol.MsgOffer, map[string]interface{}{
		"type": offer.Type.String(),
		"sdp":  offer.SDP,
	})
	offerMsg.Room = e.config.RoomID

	if err := e.config.SignalConn.WriteJSON(offerMsg); err != nil {
		return fmt.Errorf("send ICE restart offer: %w", err)
	}

	log.Info().Msg("ICE restart offer sent via signaling")
	return nil
}

// Close terminates the peer connection.
func (e *Engine) Close() error {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.closed {
		return nil
	}
	e.closed = true

	// Stop any active reconnect cycle
	e.restartMu.Lock()
	if e.restarting {
		e.restarting = false
		select {
		case e.reconnectStop <- struct{}{}:
		default:
		}
	}
	e.restartMu.Unlock()

	return e.pc.Close()
}

// ConnectionState returns the current ICE connection state.
func (e *Engine) ConnectionState() pion.ICEConnectionState {
	return e.pc.ICEConnectionState()
}

func boolPtr(b bool) *bool {
	return &b
}
