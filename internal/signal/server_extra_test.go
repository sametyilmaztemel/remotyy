package signal

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/rs/zerolog"
	"github.com/sametyilmaztemel/remotty/internal/config"
	"github.com/sametyilmaztemel/remotty/internal/logging"
	"github.com/sametyilmaztemel/remotty/internal/protocol"
)

// ======== Direct Function Tests ========

func TestHandleHeartbeat(t *testing.T) {
	s, _ := newTestServer(t)

	peer := &Peer{
		ID:            "test-peer",
		LastHeartbeat: time.Time{}, // zero value
	}

	s.handleHeartbeat(peer)

	if peer.LastHeartbeat.IsZero() {
		t.Error("handleHeartbeat should update LastHeartbeat")
	}

	if time.Since(peer.LastHeartbeat) > time.Second {
		t.Error("LastHeartbeat should be set to current time")
	}
}

func TestHandleUpdate(t *testing.T) {
	s, _ := newTestServer(t)

	peer := &Peer{
		ID: "test-peer",
		Info: protocol.HostInfo{
			ID:       "test-peer",
			Name:     "original-host",
			Features: []string{"terminal"},
		},
	}

	// Update features
	updateMsg := protocol.NewMessage(protocol.MsgUpdate, protocol.HostInfo{
		Features: []string{"terminal", "screen", "file"},
	})

	s.handleUpdate(peer, updateMsg)

	if len(peer.Info.Features) != 3 {
		t.Errorf("Features = %v, want 3 items", peer.Info.Features)
	}

	// Update with nil features should not change
	updateMsgNil := protocol.NewMessage(protocol.MsgUpdate, protocol.HostInfo{})
	s.handleUpdate(peer, updateMsgNil)
	if len(peer.Info.Features) != 3 {
		t.Errorf("Features should remain unchanged, got %v", peer.Info.Features)
	}

	// Invalid payload should not panic
	badMsg := protocol.Message{
		Type:    protocol.MsgUpdate,
		Payload: []byte(`{invalid json}`),
	}
	s.handleUpdate(peer, badMsg)
}

func TestIsOriginAllowed(t *testing.T) {
	tests := []struct {
		name    string
		origin  string
		allowed []string
		dev     bool
		want    bool
	}{
		{"dev mode no origins", "http://example.com", nil, true, true},
		{"dev mode with origins", "http://example.com", []string{"http://other.com"}, true, false},
		{"explicit match", "http://example.com", []string{"http://example.com"}, false, true},
		{"wildcard", "http://anything.com", []string{"*"}, false, true},
		{"no match", "http://evil.com", []string{"http://good.com"}, false, false},
		{"empty allowed", "http://example.com", []string{}, false, true},
		{"multiple allowed match", "http://b.com", []string{"http://a.com", "http://b.com", "http://c.com"}, false, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isOriginAllowed(tt.origin, tt.allowed, tt.dev)
			if got != tt.want {
				t.Errorf("isOriginAllowed(%q, %v, %v) = %v, want %v",
					tt.origin, tt.allowed, tt.dev, got, tt.want)
			}
		})
	}
}

// ======== HTTP Handler Tests ========

func TestHandleStatsEndpoint(t *testing.T) {
	_, ts := newTestServer(t)

	resp, err := http.Get(ts.URL + "/api/stats")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}

	var stats map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&stats); err != nil {
		t.Fatal(err)
	}

	if stats["accepting"] != true {
		t.Errorf("accepting = %v, want true", stats["accepting"])
	}
	if _, ok := stats["peers"]; !ok {
		t.Error("stats should include peers count")
	}
	if _, ok := stats["rooms"]; !ok {
		t.Error("stats should include rooms count")
	}
	if _, ok := stats["hosts"]; !ok {
		t.Error("stats should include hosts count")
	}
}

func TestHandleListHostsAPIMethodNotAllowed(t *testing.T) {
	_, ts := newTestServer(t)

	// POST instead of GET
	resp, err := http.Post(ts.URL+"/api/hosts", "application/json", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", resp.StatusCode)
	}
}

func TestShutdownWithoutHTTPServer(t *testing.T) {
	log, err := logging.Init(zerolog.Disabled, "console", "")
	if err != nil {
		t.Fatal(err)
	}
	s := NewServer(config.SignalConfig{}, log)

	// Shutdown with no httpServer should not panic or error
	if err := s.Shutdown(); err != nil {
		t.Errorf("Shutdown with no httpServer: %v", err)
	}
}

// ======== Middleware Tests ========

func TestMiddlewareCORSBlockedOrigin(t *testing.T) {
	_, ts := newTestServerWithConfig(t, config.SignalConfig{
		DevMode:        true,
		AllowedOrigins: []string{"http://allowed.com"},
	})

	req, _ := http.NewRequest("GET", ts.URL+"/health", nil)
	req.Header.Set("Origin", "http://evil.com")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("expected 403 for blocked origin, got %d", resp.StatusCode)
	}
}

func TestMiddlewareCORSOptionsPreflight(t *testing.T) {
	_, ts := newTestServerWithConfig(t, config.SignalConfig{
		DevMode:        true,
		AllowedOrigins: []string{"http://example.com"},
	})

	req, _ := http.NewRequest("OPTIONS", ts.URL+"/health", nil)
	req.Header.Set("Origin", "http://example.com")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200 for OPTIONS, got %d", resp.StatusCode)
	}

	if resp.Header.Get("Access-Control-Allow-Origin") != "http://example.com" {
		t.Errorf("Access-Control-Allow-Origin = %q, want %q",
			resp.Header.Get("Access-Control-Allow-Origin"), "http://example.com")
	}
}

func TestMiddlewareCORSDevModeWildcard(t *testing.T) {
	_, ts := newTestServer(t) // DevMode: true, no explicit origins

	req, _ := http.NewRequest("GET", ts.URL+"/health", nil)
	req.Header.Set("Origin", "http://any-origin.com")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200 for any origin in dev mode, got %d", resp.StatusCode)
	}

	if resp.Header.Get("Access-Control-Allow-Origin") != "*" {
		t.Errorf("expected * CORS header in dev mode, got %q",
			resp.Header.Get("Access-Control-Allow-Origin"))
	}
}

func TestMiddlewareRateLimit(t *testing.T) {
	_, ts := newTestServerWithConfig(t, config.SignalConfig{
		DevMode:   true,
		RateLimit: 1, // 1 request per minute
	})

	// First request should succeed
	resp, err := http.Get(ts.URL + "/health")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("first request: expected 200, got %d", resp.StatusCode)
	}

	// Second request quickly should be rate limited
	resp, err = http.Get(ts.URL + "/health")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusTooManyRequests {
		t.Errorf("rate limited request: expected 429, got %d", resp.StatusCode)
	}
}

// ======== Heartbeat via WebSocket ========

func TestHeartbeatViaWebSocket(t *testing.T) {
	s, ts := newTestServer(t)

	conn := dialWS(t, wsURL(ts))
	defer conn.Close()

	// Register a host
	conn.WriteJSON(protocol.NewMessage(protocol.MsgRegister, protocol.RegisterPayload{
		Name: "hb-host", Platform: "linux", Arch: "arm64",
	}))
	var regResp protocol.Message
	conn.ReadJSON(&regResp)

	// Find the peer
	s.mu.RLock()
	var peer *Peer
	for _, p := range s.peers {
		peer = p
		break
	}
	s.mu.RUnlock()

	if peer == nil {
		t.Fatal("no peer found")
	}

	oldHeartbeat := peer.LastHeartbeat

	// Send heartbeat
	conn.WriteJSON(protocol.NewMessage(protocol.MsgHeartbeat, nil))

	// Give it a moment to process
	time.Sleep(50 * time.Millisecond)

	peer.Mu.Lock()
	newHeartbeat := peer.LastHeartbeat
	peer.Mu.Unlock()

	if !newHeartbeat.After(oldHeartbeat) {
		t.Error("heartbeat should update LastHeartbeat")
	}
}

// ======== Ping/Pong ========

func TestPingPong(t *testing.T) {
	_, ts := newTestServer(t)

	conn := dialWS(t, wsURL(ts))
	defer conn.Close()

	// Send ping
	conn.WriteJSON(protocol.NewMessage(protocol.MsgPing, nil))

	// Expect pong
	conn.SetReadDeadline(time.Now().Add(time.Second))
	var resp protocol.Message
	if err := conn.ReadJSON(&resp); err != nil {
		t.Fatalf("ReadJSON: %v", err)
	}
	if resp.Type != protocol.MsgPong {
		t.Errorf("expected pong, got %s", resp.Type)
	}
}

// ======== WebSocket error cases ========

func TestWebSocketPayloadTooLarge(t *testing.T) {
	_, ts := newTestServer(t)

	conn := dialWS(t, wsURL(ts))
	defer conn.Close()

	// Register first
	conn.WriteJSON(protocol.NewMessage(protocol.MsgRegister, protocol.RegisterPayload{
		Name: "big-payload-host", Platform: "linux",
	}))
	var regResp protocol.Message
	conn.ReadJSON(&regResp)

	// Send a message with a very large payload (exceeds maxPayloadSize = 1MB)
	// Use raw text message to send a large payload
	bigData := strings.Repeat("x", maxPayloadSize+1)
	bigMsg := protocol.NewMessage(protocol.MsgUpdate, map[string]string{"data": bigData})
	conn.WriteJSON(bigMsg)

	// Should get an error response (payload_too_large)
	conn.SetReadDeadline(time.Now().Add(time.Second))
	var resp protocol.Message
	if err := conn.ReadJSON(&resp); err != nil {
		t.Fatalf("ReadJSON: %v", err)
	}
	if resp.Type != protocol.MsgError {
		t.Errorf("expected error for large payload, got %s", resp.Type)
	}
}

func TestWebSocketInvalidMessage(t *testing.T) {
	_, ts := newTestServer(t)

	conn := dialWS(t, wsURL(ts))
	defer conn.Close()

	// Send raw invalid JSON (not a valid protocol.Message)
	if err := conn.WriteMessage(websocket.TextMessage, []byte(`{invalid json}`)); err != nil {
		t.Fatal(err)
	}

	// Should get an error response
	conn.SetReadDeadline(time.Now().Add(time.Second))
	var resp protocol.Message
	if err := conn.ReadJSON(&resp); err != nil {
		t.Logf("ReadJSON (expected timeout or error): %v", err)
	}
}

// ======== Connect by Hostname ========

func TestConnectByHostName(t *testing.T) {
	_, ts := newTestServer(t)

	// Register host with a name
	host := dialWS(t, wsURL(ts))
	defer host.Close()
	host.WriteJSON(protocol.NewMessage(protocol.MsgRegister, protocol.RegisterPayload{
		Name: "named-host", Platform: "linux", Arch: "arm64",
	}))
	var regResp protocol.Message
	host.ReadJSON(&regResp)

	// Client connects using host name instead of ID
	client := dialWS(t, wsURL(ts))
	defer client.Close()
	client.WriteJSON(protocol.NewMessage(protocol.MsgConnect, protocol.ConnectPayload{
		HostID: "named-host",
	}))

	var connectResp protocol.Message
	if err := client.ReadJSON(&connectResp); err != nil {
		t.Fatal(err)
	}
	if connectResp.Type != protocol.MsgRoomReady {
		t.Errorf("expected room_ready, got %s", connectResp.Type)
	}
}

// ======== Host with Too Many Features ========

func TestRegisterTooManyFeatures(t *testing.T) {
	_, ts := newTestServer(t)
	conn := dialWS(t, wsURL(ts))
	defer conn.Close()

	features := make([]string, maxFeaturesCount+1)
	for i := range features {
		features[i] = fmt.Sprintf("feature-%d", i)
	}

	conn.WriteJSON(protocol.NewMessage(protocol.MsgRegister, protocol.RegisterPayload{
		Name: "feature-heavy", Platform: "linux", Arch: "arm64",
		Features: features,
	}))

	var resp protocol.Message
	if err := conn.ReadJSON(&resp); err != nil {
		t.Fatal(err)
	}
	if resp.Type != protocol.MsgError {
		t.Errorf("expected error for too many features, got %s", resp.Type)
	}
}

// ======== Server with WebDir not found ========

func TestServerStartWebDirNotFound(t *testing.T) {
	log, err := logging.Init(zerolog.Disabled, "console", "")
	if err != nil {
		t.Fatal(err)
	}
	s := NewServer(config.SignalConfig{
		DevMode: true,
		WebDir:  "/nonexistent/webdir",
	}, log)

	handler := s.HTTPHandler()
	ts := httptest.NewServer(handler)
	defer ts.Close()

	// Server should still work even with missing web dir
	resp, err := http.Get(ts.URL + "/health")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
	s.Shutdown()
}
