// Package signal implements the WebSocket signaling server for WebRTC negotiation.
// The signaling server is a blind relay — it coordinates connections but never sees
// terminal or screen data.
package signal

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/sametyilmaztemel/remotty/internal/config"
	"github.com/sametyilmaztemel/remotty/internal/logging"
	"github.com/sametyilmaztemel/remotty/internal/protocol"
)

const (
	maxHostNameLen      = 64
	maxPayloadSize      = 1 << 20 // 1MB max incoming message
	maxFeaturesCount    = 20
	peerReadDeadline    = 45 * time.Second // 3x heartbeat interval (15s)
)

var upgrader = websocket.Upgrader{
	CheckOrigin:       func(r *http.Request) bool { return true },
	ReadBufferSize:    4096,
	WriteBufferSize:   4096,
	EnableCompression: true,
}

// PeerState tracks the lifecycle of a connected peer.
type PeerState int

const (
	PeerNew PeerState = iota
	PeerRegistered
	PeerInRoom
	PeerDisconnected
)

// Peer represents a connected WebSocket peer (host or client).
type Peer struct {
	ID         string
	Role       string // "host" or "client"
	Conn       *websocket.Conn
	RemoteAddr string
	State      PeerState
	Info       protocol.HostInfo
	RoomID     string
	ConnectedAt time.Time
	LastHeartbeat time.Time
	Mu         sync.Mutex
	log        zerolog.Logger
}

// Room pairs a host and client for WebRTC negotiation.
type Room struct {
	ID        string
	Host      *Peer
	Client    *Peer
	CreatedAt time.Time
	mu        sync.Mutex
}

// Server is the WebSocket signaling server.
type Server struct {
	cfg        config.SignalConfig
	httpServer *http.Server
	peers      map[string]*Peer
	rooms      map[string]*Room
	hostIndex  map[string]*Peer
	mu         sync.RWMutex
	peerCount  atomic.Int64
	roomCount  atomic.Int64
	accepting  atomic.Bool
	rateLimit  *RateLimiter
	log        *logging.Logger
	done       chan struct{}
}

// NewServer creates a signaling server.
func NewServer(cfg config.SignalConfig, log *logging.Logger) *Server {
	s := &Server{
		cfg:       cfg,
		peers:     make(map[string]*Peer),
		rooms:     make(map[string]*Room),
		hostIndex: make(map[string]*Peer),
		log:       log,
		done:      make(chan struct{}),
	}
	s.accepting.Store(true)

	// Initialize rate limiter if configured
	if cfg.RateLimit > 0 {
		s.rateLimit = NewRateLimiter(cfg.RateLimit)
	}

	return s
}

// Start begins listening for WebSocket connections.
func (s *Server) Start(ctx context.Context) error {
	s.accepting.Store(true)

	mux := http.NewServeMux()
	mux.HandleFunc("/ws", s.handleWebSocket)
	mux.HandleFunc("/health", s.handleHealth)
	mux.HandleFunc("/api/hosts", s.handleListHostsAPI)
	mux.HandleFunc("/api/stats", s.handleStats)

	// Serve web UI if configured
	if s.cfg.WebDir != "" {
		if info, err := os.Stat(s.cfg.WebDir); err == nil && info.IsDir() {
			fileServer := http.FileServer(http.Dir(s.cfg.WebDir))
			mux.Handle("/", fileServer)
			s.log.Info().Str("web_dir", s.cfg.WebDir).Msg("Serving web UI")
		} else {
			s.log.Warn().Str("web_dir", s.cfg.WebDir).Err(err).Msg("Web UI directory not found")
		}
	}

	addr := s.cfg.Addr()
	s.httpServer = &http.Server{
		Addr:         addr,
		Handler:      withMiddleware(mux, s.cfg.DevMode, s.rateLimit, s.cfg.AllowedOrigins),
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
		ConnContext:  func(ctx context.Context, c net.Conn) context.Context { return ctx },
	}

	// Start listening
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("listen on %s: %w", addr, err)
	}

	s.log.Info().
		Str("addr", addr).
		Bool("tls", s.cfg.TLS.Enabled).
		Bool("dev_mode", s.cfg.DevMode).
		Int("rate_limit", s.cfg.RateLimit).
		Msg("Signaling server starting")

	go func() {
		if s.cfg.TLS.Enabled {
			err = s.httpServer.ServeTLS(listener, s.cfg.TLS.CertFile, s.cfg.TLS.KeyFile)
		} else {
			err = s.httpServer.Serve(listener)
		}
		if err != nil && err != http.ErrServerClosed {
			s.log.Fatal().Err(err).Msg("Server error")
		}
	}()

	// Wait for shutdown
	<-ctx.Done()
	return s.Shutdown()
}

// Shutdown gracefully stops the server.
func (s *Server) Shutdown() error {
	s.accepting.Store(false)
	if s.httpServer != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return s.httpServer.Shutdown(ctx)
	}
	return nil
}

// handleWebSocket upgrades HTTP to WebSocket.
func (s *Server) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	if !s.accepting.Load() {
		http.Error(w, "Server shutting down", http.StatusServiceUnavailable)
		return
	}

	// Auth token validation (if configured)
	if s.cfg.AuthToken != "" {
		token := r.URL.Query().Get("token")
		if token == "" {
			// Also check Authorization header
			auth := r.Header.Get("Authorization")
			if strings.HasPrefix(auth, "Bearer ") {
				token = strings.TrimPrefix(auth, "Bearer ")
			}
		}
		if token != s.cfg.AuthToken {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			s.log.Audit.Log("auth_rejected", "", "", r.RemoteAddr,
				"WebSocket auth token rejected", false)
			log.Warn().Str("remote", r.RemoteAddr).Msg("WebSocket auth rejected")
			return
		}
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Warn().Err(err).Msg("WebSocket upgrade failed")
		return
	}

	peer := &Peer{
		ID:            fmt.Sprintf("p-%d", time.Now().UnixNano()),
		Conn:          conn,
		RemoteAddr:    r.RemoteAddr,
		State:         PeerNew,
		ConnectedAt:   time.Now(),
		LastHeartbeat: time.Now(),
	}

	peer.log = log.With().Str("peer", peer.ID).Str("remote", r.RemoteAddr).Logger()

	s.log.Audit.Log("ws_connect", peer.ID, "", peer.RemoteAddr,
		fmt.Sprintf("path=/ws"), true)

	peer.log.Info().Msg("New WebSocket connection")

	s.mu.Lock()
	s.peers[peer.ID] = peer
	s.peerCount.Add(1)
	s.mu.Unlock()

	defer s.cleanupPeer(peer)
	s.readLoop(peer)
}

// readLoop processes incoming WebSocket messages.
func (s *Server) readLoop(peer *Peer) {
	// Set initial read deadline
	peer.Conn.SetReadDeadline(time.Now().Add(peerReadDeadline))

	for {
		_, data, err := peer.Conn.ReadMessage()
		if err != nil {
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				peer.log.Warn().Msg("Peer heartbeat timeout, closing connection")
			} else if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseNormalClosure) {
				peer.log.Debug().Err(err).Msg("WebSocket read error")
			}
			return
		}

		// Reset deadline on any successful read
		peer.Conn.SetReadDeadline(time.Now().Add(peerReadDeadline))

		if len(data) > maxPayloadSize {
			peer.log.Warn().Int("size", len(data)).Msg("Message too large, dropping")
			s.sendError(peer, "payload_too_large", fmt.Sprintf("Message exceeds %d bytes", maxPayloadSize))
			continue
		}

		var msg protocol.Message
		if err := json.Unmarshal(data, &msg); err != nil {
			peer.log.Warn().Err(err).Msg("Invalid message format")
			s.sendError(peer, "invalid_message", "Malformed JSON")
			continue
		}

		s.routeMessage(peer, msg)
	}
}

// routeMessage dispatches a message based on its type.
func (s *Server) routeMessage(peer *Peer, msg protocol.Message) {
	peer.log.Debug().Str("type", string(msg.Type)).Msg("Received message")

	switch msg.Type {
	case protocol.MsgRegister:
		s.handleRegister(peer, msg)
	case protocol.MsgHeartbeat:
		s.handleHeartbeat(peer)
	case protocol.MsgUpdate:
		s.handleUpdate(peer, msg)
	case protocol.MsgListHosts:
		s.handleListHosts(peer)
	case protocol.MsgConnect:
		s.handleConnect(peer, msg)
	case protocol.MsgOffer, protocol.MsgAnswer, protocol.MsgICECandidate:
		s.relayToRoom(peer, msg)
	case protocol.MsgPing:
		s.sendMessage(peer, protocol.NewMessage(protocol.MsgPong, nil))
	default:
		peer.log.Warn().Str("type", string(msg.Type)).Msg("Unknown message type")
	}
}

func (s *Server) handleRegister(peer *Peer, msg protocol.Message) {
	if peer.State != PeerNew {
		s.sendError(peer, "invalid_state", "Already registered")
		return
	}

	var reg protocol.RegisterPayload
	if err := json.Unmarshal(msg.Payload, &reg); err != nil {
		s.sendError(peer, "invalid_payload", "Cannot parse registration")
		return
	}

	// Validate registration fields
	reg.Name = strings.TrimSpace(reg.Name)
	if reg.Name == "" {
		s.sendError(peer, "invalid_payload", "Host name is required")
		return
	}
	if len(reg.Name) > maxHostNameLen {
		s.sendError(peer, "invalid_payload", fmt.Sprintf("Host name too long (max %d chars)", maxHostNameLen))
		return
	}
	if len(reg.Features) > maxFeaturesCount {
		s.sendError(peer, "invalid_payload", fmt.Sprintf("Too many features (max %d)", maxFeaturesCount))
		return
	}

	peer.Role = "host"
	peer.State = PeerRegistered
	peer.Info = protocol.HostInfo{
		ID:       peer.ID,
		Name:     reg.Name,
		Platform: reg.Platform,
		Arch:     reg.Arch,
		Version:  reg.Version,
		Online:   true,
		Features: reg.Features,
	}

	s.mu.Lock()
	s.hostIndex[reg.Name] = peer
	s.mu.Unlock()

	s.log.Audit.Log("host_register", peer.ID, "", peer.RemoteAddr,
		fmt.Sprintf("host=%s platform=%s/%s", reg.Name, reg.Platform, reg.Arch), true)

	peer.log.Info().
		Str("name", reg.Name).
		Str("platform", reg.Platform).
		Strs("features", reg.Features).
		Msg("Host registered")

	s.sendMessage(peer, protocol.NewMessage(protocol.MsgRegister, map[string]interface{}{
		"id":     peer.ID,
		"status": "ok",
	}))
}

func (s *Server) handleHeartbeat(peer *Peer) {
	peer.Mu.Lock()
	peer.LastHeartbeat = time.Now()
	peer.Mu.Unlock()
}

func (s *Server) handleUpdate(peer *Peer, msg protocol.Message) {
	var update protocol.HostInfo
	if err := json.Unmarshal(msg.Payload, &update); err != nil {
		return
	}
	peer.Mu.Lock()
	if update.Features != nil {
		peer.Info.Features = update.Features
	}
	peer.Mu.Unlock()
}

func (s *Server) handleListHosts(peer *Peer) {
	if peer.Role == "" {
		peer.Role = "client"
	}
	peer.State = PeerRegistered

	s.mu.RLock()
	hosts := make([]protocol.HostInfo, 0, len(s.hostIndex))
	for _, p := range s.hostIndex {
		if p.State == PeerRegistered || p.State == PeerInRoom {
			hosts = append(hosts, p.Info)
		}
	}
	s.mu.RUnlock()

	s.sendMessage(peer, protocol.NewMessage(protocol.MsgHostList, map[string]interface{}{
		"hosts": hosts,
	}))
}

func (s *Server) handleConnect(peer *Peer, msg protocol.Message) {
	var req protocol.ConnectPayload
	if err := json.Unmarshal(msg.Payload, &req); err != nil {
		s.sendError(peer, "invalid_payload", "Cannot parse connect request")
		return
	}

	if peer.Role == "" {
		peer.Role = "client"
	}

	// Find host
	s.mu.RLock()
	host, ok := s.peers[req.HostID]
	if !ok {
		// Try by hostname
		if h, found := s.hostIndex[req.HostID]; found {
			host = h
		}
	}
	s.mu.RUnlock()

	if host == nil || host.State == PeerDisconnected {
		s.sendError(peer, "host_not_found", fmt.Sprintf("Host %s is offline", req.HostID))
		return
	}

	// Create room
	roomID := fmt.Sprintf("r-%d", time.Now().UnixNano())
	room := &Room{
		ID:        roomID,
		Host:      host,
		Client:    peer,
		CreatedAt: time.Now(),
	}

	s.mu.Lock()
	s.rooms[roomID] = room
	host.RoomID = roomID
	host.State = PeerInRoom
	peer.RoomID = roomID
	peer.State = PeerInRoom
	s.roomCount.Add(1)
	s.mu.Unlock()

	s.log.Audit.Log("room_created", peer.ID, roomID, peer.RemoteAddr,
		fmt.Sprintf("host=%s client connected", host.Info.Name), true)

	log.Info().
		Str("room", roomID).
		Str("host", host.Info.Name).
		Str("client", peer.ID).
		Msg("Room created")

	// Tell client the room is ready
	s.sendMessage(peer, protocol.NewMessage(protocol.MsgRoomReady, map[string]interface{}{
		"room":    roomID,
		"host_id": host.ID,
		"host":    host.Info,
	}))

	// Notify host about incoming client
	s.sendMessage(host, protocol.NewMessage(protocol.MsgConnect, map[string]interface{}{
		"room":      roomID,
		"client_id": peer.ID,
	}))
}

func (s *Server) relayToRoom(sender *Peer, msg protocol.Message) {
	s.mu.RLock()
	room, ok := s.rooms[sender.RoomID]
	s.mu.RUnlock()

	if !ok || room == nil {
		s.sendError(sender, "no_room", "You are not in a room")
		return
	}

	var target *Peer
	if sender.ID == room.Host.ID {
		target = room.Client
	} else {
		target = room.Host
	}

	if target == nil {
		return
	}

	msg.Room = room.ID
	s.sendMessage(target, msg)
}

func (s *Server) cleanupPeer(peer *Peer) {
	s.mu.Lock()

	// Remove from room if active
	if room, ok := s.rooms[peer.RoomID]; ok && room != nil {
		var other *Peer
		if peer.ID == room.Host.ID {
			other = room.Client
		} else {
			other = room.Host
		}
		if other != nil {
			other.RoomID = ""
			other.State = PeerRegistered
			s.sendMessage(other, protocol.NewMessage(protocol.MsgPeerLeft, map[string]interface{}{
				"peer_id": peer.ID,
				"reason":  "disconnected",
			}))
		}
		delete(s.rooms, peer.RoomID)
		s.roomCount.Add(-1)
	}

	// Remove from host index
	for name, p := range s.hostIndex {
		if p.ID == peer.ID {
			delete(s.hostIndex, name)
			break
		}
	}

	delete(s.peers, peer.ID)
	s.peerCount.Add(-1)
	s.mu.Unlock()

	peer.State = PeerDisconnected

	s.log.Audit.Log("peer_disconnect", peer.ID, "", peer.RemoteAddr,
		fmt.Sprintf("role=%s", peer.Role), true)
	if peer.RoomID != "" {
		s.log.Audit.Log("room_destroyed", peer.ID, peer.RoomID, peer.RemoteAddr,
			fmt.Sprintf("role=%s cleanup", peer.Role), true)
	}

	log.Info().
		Str("peer", peer.ID).
		Str("role", peer.Role).
		Msg("Peer disconnected")
}

// ======== HTTP Handlers ========

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	s.mu.RLock()
	hostCount := len(s.hostIndex)
	roomCount := len(s.rooms)
	peerCount := len(s.peers)
	s.mu.RUnlock()
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":  "ok",
		"version": config.Version,
		"hosts":   hostCount,
		"rooms":   roomCount,
		"peers":   peerCount,
		"uptime":  time.Since(startTime).String(),
	})
}

var startTime = time.Now()

func (s *Server) handleListHostsAPI(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	s.mu.RLock()
	hosts := make([]protocol.HostInfo, 0, len(s.hostIndex))
	for _, p := range s.hostIndex {
		hosts = append(hosts, p.Info)
	}
	s.mu.RUnlock()
	json.NewEncoder(w).Encode(hosts)
}

func (s *Server) handleStats(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	s.mu.RLock()
	info := map[string]interface{}{
		"peers":       s.peerCount.Load(),
		"rooms":       s.roomCount.Load(),
		"hosts":       len(s.hostIndex),
		"accepting":   s.accepting.Load(),
	}
	s.mu.RUnlock()
	json.NewEncoder(w).Encode(info)
}

// ======== Helpers ========

func (s *Server) sendMessage(peer *Peer, msg protocol.Message) {
	peer.Mu.Lock()
	defer peer.Mu.Unlock()
	if err := peer.Conn.WriteJSON(msg); err != nil {
		peer.log.Warn().Err(err).Msg("Failed to send message")
	}
}

func (s *Server) sendError(peer *Peer, code string, message string) {
	var errCode int
	switch code {
	case "unauthorized":
		errCode = protocol.ErrUnauthorized
	case "invalid_state":
		errCode = protocol.ErrInvalidState
	case "invalid_payload", "invalid_message":
		errCode = protocol.ErrInvalidPayload
	case "payload_too_large":
		errCode = protocol.ErrPayloadTooLarge
	case "host_not_found":
		errCode = protocol.ErrHostNotFound
	case "no_room":
		errCode = protocol.ErrRoomNotFound
	default:
		errCode = protocol.ErrInternal
	}
	s.sendMessage(peer, protocol.NewMessage(protocol.MsgError, protocol.ErrorPayload{
		Code:    errCode,
		Message: fmt.Sprintf("%s: %s", code, message),
	}))
}

// HTTPHandler returns the HTTP mux for mounting in a custom server.
func (s *Server) HTTPHandler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/ws", s.handleWebSocket)
	mux.HandleFunc("/health", s.handleHealth)
	mux.HandleFunc("/api/hosts", s.handleListHostsAPI)
	mux.HandleFunc("/api/stats", s.handleStats)
	return withMiddleware(mux, s.cfg.DevMode, s.rateLimit, s.cfg.AllowedOrigins)
}

// withMiddleware adds CORS, rate limiting, and logging middleware.
func withMiddleware(next http.Handler, dev bool, rl *RateLimiter, allowedOrigins []string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// CORS origin check
		origin := r.Header.Get("Origin")
		if origin != "" && !isOriginAllowed(origin, allowedOrigins, dev) {
			http.Error(w, "Forbidden: origin not allowed", http.StatusForbidden)
			return
		}

		// CORS headers — allow all when no specific origins configured (LAN tool default)
		if len(allowedOrigins) > 0 {
			w.Header().Set("Access-Control-Allow-Origin", allowedOrigins[0])
		} else {
			w.Header().Set("Access-Control-Allow-Origin", "*")
		}
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}

		// Rate limiting
		if rl != nil {
			ip := extractIP(r.RemoteAddr)
			if !rl.Allow(ip) {
				log.Warn().Str("ip", ip).Str("path", r.URL.Path).Msg("Rate limit exceeded")
				http.Error(w, "Too Many Requests", http.StatusTooManyRequests)
				return
			}
		}

		// Logging
		start := time.Now()
		next.ServeHTTP(w, r)
		log.Debug().
			Str("method", r.Method).
			Str("path", r.URL.Path).
			Str("remote", r.RemoteAddr).
			Dur("dur", time.Since(start)).
			Msg("HTTP request")
	})
}

// isOriginAllowed checks if the request origin is in the allowed list.
// When no origins are configured, all origins are allowed (LAN tool default).
func isOriginAllowed(origin string, allowed []string, dev bool) bool {
	if len(allowed) == 0 {
		return true // No explicit origins = allow all (any mode)
	}
	for _, a := range allowed {
		if a == "*" || a == origin {
			return true
		}
	}
	return false
}
