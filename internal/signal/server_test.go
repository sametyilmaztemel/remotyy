package signal

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gorilla/websocket"
	"github.com/rs/zerolog"
	"github.com/sametyilmaztemel/remotty/internal/config"
	"github.com/sametyilmaztemel/remotty/internal/logging"
	"github.com/sametyilmaztemel/remotty/internal/protocol"
)

func newTestServer(t *testing.T) (*Server, *httptest.Server) {
	t.Helper()
	return newTestServerWithConfig(t, config.SignalConfig{DevMode: true})
}

func newTestServerWithConfig(t *testing.T, cfg config.SignalConfig) (*Server, *httptest.Server) {
	t.Helper()
	log, err := logging.Init(zerolog.Disabled, "console", "")
	if err != nil {
		t.Fatal(err)
	}
	s := NewServer(cfg, log)
	ts := httptest.NewServer(s.HTTPHandler())
	t.Cleanup(func() {
		ts.Close()
		s.Shutdown()
	})
	return s, ts
}

func wsURL(ts *httptest.Server) string {
	u := ts.URL
	u = "ws" + strings.TrimPrefix(u, "http") + "/ws"
	return u
}

func dialWS(t *testing.T, url string) *websocket.Conn {
	t.Helper()
	c, _, err := (&websocket.Dialer{}).Dial(url, nil)
	if err != nil {
		t.Fatalf("dial %s: %v", url, err)
	}
	return c
}

func TestHealthEndpoint(t *testing.T) {
	_, ts := newTestServer(t)
	resp, err := http.Get(ts.URL + "/health")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
	var m map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&m)
	if m["status"] != "ok" {
		t.Errorf("status not ok: %v", m["status"])
	}
}

func TestApiHosts(t *testing.T) {
	_, ts := newTestServer(t)
	resp, err := http.Get(ts.URL + "/api/hosts")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

func TestWebSocketRegisterHost(t *testing.T) {
	_, ts := newTestServer(t)
	conn := dialWS(t, wsURL(ts))
	defer conn.Close()

	conn.WriteJSON(protocol.NewMessage(protocol.MsgRegister, protocol.RegisterPayload{
		Name: "test-host", Platform: "linux", Arch: "arm64",
		Features: []string{"terminal"},
	}))

	var resp protocol.Message
	if err := conn.ReadJSON(&resp); err != nil {
		t.Fatal(err)
	}
	if resp.Type != protocol.MsgRegister {
		t.Errorf("expected register, got %s", resp.Type)
	}
}

func TestWebSocketListHosts(t *testing.T) {
	_, ts := newTestServer(t)

	// Register host
	host := dialWS(t, wsURL(ts))
	defer host.Close()
	host.WriteJSON(protocol.NewMessage(protocol.MsgRegister,
		protocol.RegisterPayload{Name: "list-test", Platform: "darwin", Arch: "arm64"}))
	var regResp protocol.Message
	host.ReadJSON(&regResp)

	// List hosts
	client := dialWS(t, wsURL(ts))
	defer client.Close()
	client.WriteJSON(protocol.NewMessage(protocol.MsgListHosts, nil))

	var listResp protocol.Message
	if err := client.ReadJSON(&listResp); err != nil {
		t.Fatal(err)
	}
	if listResp.Type != protocol.MsgHostList {
		t.Fatalf("expected host_list, got %s", listResp.Type)
	}
	var hosts struct {
		Hosts []protocol.HostInfo `json:"hosts"`
	}
	if err := json.Unmarshal(listResp.Payload, &hosts); err != nil {
		t.Fatal(err)
	}
	if len(hosts.Hosts) != 1 {
		t.Errorf("expected 1 host, got %d", len(hosts.Hosts))
	}
	if hosts.Hosts[0].Name != "list-test" {
		t.Errorf("expected name list-test, got %s", hosts.Hosts[0].Name)
	}
}

func TestWebSocketConnectFlow(t *testing.T) {
	_, ts := newTestServer(t)

	// Host registers
	host := dialWS(t, wsURL(ts))
	defer host.Close()
	host.WriteJSON(protocol.NewMessage(protocol.MsgRegister,
		protocol.RegisterPayload{Name: "connect-test", Platform: "linux", Arch: "arm64"}))
	var regResp protocol.Message
	if err := host.ReadJSON(&regResp); err != nil {
		t.Fatal(err)
	}
	t.Logf("Host registered: %s", string(regResp.Payload))

	// Client lists hosts
	client := dialWS(t, wsURL(ts))
	defer client.Close()

	client.WriteJSON(protocol.NewMessage(protocol.MsgListHosts, nil))
	var listResp protocol.Message
	if err := client.ReadJSON(&listResp); err != nil {
		t.Fatal(err)
	}
	var hosts struct {
		Hosts []protocol.HostInfo `json:"hosts"`
	}
	if err := json.Unmarshal(listResp.Payload, &hosts); err != nil {
		t.Fatal(err)
	}
	t.Logf("Found %d hosts", len(hosts.Hosts))
	if len(hosts.Hosts) == 0 {
		t.Skip("no hosts available")
	}

	// Connect to first host
	t.Logf("Connecting to host: %s (ID: %s)", hosts.Hosts[0].Name, hosts.Hosts[0].ID)
	client.WriteJSON(protocol.NewMessage(protocol.MsgConnect, protocol.ConnectPayload{
		HostID: hosts.Hosts[0].ID,
	}))

	var roomResp protocol.Message
	if err := client.ReadJSON(&roomResp); err != nil {
		t.Fatal(err)
	}
	if roomResp.Type != protocol.MsgRoomReady {
		t.Errorf("expected room_ready, got %s", roomResp.Type)
	}

	// Host should get connect notification
	var hostNotif protocol.Message
	if err := host.ReadJSON(&hostNotif); err != nil {
		t.Fatal(err)
	}
	if hostNotif.Type != protocol.MsgConnect {
		t.Errorf("expected connect notification, got %s", hostNotif.Type)
	}
}

func TestWebRTCSignaling(t *testing.T) {
	_, ts := newTestServer(t)

	// Setup host and client
	host := dialWS(t, wsURL(ts))
	defer host.Close()
	host.WriteJSON(protocol.NewMessage(protocol.MsgRegister,
		protocol.RegisterPayload{Name: "webrtc-test", Platform: "linux", Arch: "arm64"}))
	var regResp protocol.Message
	host.ReadJSON(&regResp)

	client := dialWS(t, wsURL(ts))
	defer client.Close()
	client.WriteJSON(protocol.NewMessage(protocol.MsgListHosts, nil))
	var listResp protocol.Message
	client.ReadJSON(&listResp)
	var hosts struct {
		Hosts []protocol.HostInfo `json:"hosts"`
	}
	json.Unmarshal(listResp.Payload, &hosts)
	if len(hosts.Hosts) == 0 {
		t.Skip("no hosts")
	}

	client.WriteJSON(protocol.NewMessage(protocol.MsgConnect,
		protocol.ConnectPayload{HostID: hosts.Hosts[0].ID}))
	var roomResp protocol.Message
	client.ReadJSON(&roomResp)

	// Simulate WebRTC offer/answer exchange
	var hostNotif protocol.Message
	host.ReadJSON(&hostNotif)
	roomID := hostNotif.Room
	if roomID == "" {
		// Get room from payload
		var payload struct {
			Room string `json:"room"`
		}
		json.Unmarshal(hostNotif.Payload, &payload)
		roomID = payload.Room
	}
	t.Logf("Room ID: %s", roomID)

	// Host sends offer
	offerPayload := map[string]string{"type": "offer", "sdp": "test-sdp-offer"}
	host.WriteJSON(protocol.Message{
		Type:    protocol.MsgOffer,
		Payload: toRaw(offerPayload),
		Room:    roomID,
	})

	// Client receives offer
	var offerMsg protocol.Message
	if err := client.ReadJSON(&offerMsg); err != nil {
		t.Fatal(err)
	}
	if offerMsg.Type != protocol.MsgOffer {
		t.Errorf("expected offer, got %s", offerMsg.Type)
	}

	// Client sends answer
	answerPayload := map[string]string{"type": "answer", "sdp": "test-sdp-answer"}
	client.WriteJSON(protocol.Message{
		Type:    protocol.MsgAnswer,
		Payload: toRaw(answerPayload),
		Room:    roomID,
	})

	// Host receives answer
	var answerMsg protocol.Message
	if err := host.ReadJSON(&answerMsg); err != nil {
		t.Fatal(err)
	}
	if answerMsg.Type != protocol.MsgAnswer {
		t.Errorf("expected answer, got %s", answerMsg.Type)
	}

	// ICE candidate exchange
	icePayload := map[string]interface{}{
		"candidate":     "candidate:1 1 UDP 2122252543 192.168.1.1 12345 typ host",
		"sdpMid":        "0",
		"sdpMLineIndex": 0,
	}
	host.WriteJSON(protocol.Message{
		Type:    protocol.MsgICECandidate,
		Payload: toRaw(icePayload),
		Room:    roomID,
	})

	var iceMsg protocol.Message
	if err := client.ReadJSON(&iceMsg); err != nil {
		t.Fatal(err)
	}
	if iceMsg.Type != protocol.MsgICECandidate {
		t.Errorf("expected ice_candidate, got %s", iceMsg.Type)
	}
}

func TestMultipleHosts(t *testing.T) {
	_, ts := newTestServer(t)

	// Register 3 hosts
	for i := 0; i < 3; i++ {
		conn := dialWS(t, wsURL(ts))
		defer conn.Close()
		name := fmt.Sprintf("host-%d", i)
		conn.WriteJSON(protocol.NewMessage(protocol.MsgRegister,
			protocol.RegisterPayload{Name: name, Platform: "linux", Arch: "arm64"}))
		var resp protocol.Message
		conn.ReadJSON(&resp)
	}

	// Verify all 3 appear in host list
	client := dialWS(t, wsURL(ts))
	defer client.Close()
	client.WriteJSON(protocol.NewMessage(protocol.MsgListHosts, nil))
	var listResp protocol.Message
	client.ReadJSON(&listResp)

	var hosts struct {
		Hosts []protocol.HostInfo `json:"hosts"`
	}
	json.Unmarshal(listResp.Payload, &hosts)
	if len(hosts.Hosts) != 3 {
		t.Errorf("expected 3 hosts, got %d", len(hosts.Hosts))
	}
}

func toRaw(v interface{}) json.RawMessage {
	data, _ := json.Marshal(v)
	return data
}

// ======== Auth Token Tests ========

func TestAuthTokenRequired(t *testing.T) {
	_, ts := newTestServerWithConfig(t, config.SignalConfig{
		DevMode:   true,
		AuthToken: "secret-token-123",
	})

	// Connection without token should fail
	_, resp, err := (&websocket.Dialer{}).Dial(wsURL(ts), nil)
	if err == nil {
		t.Fatal("expected auth rejection, but connection succeeded")
	}
	if resp != nil && resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", resp.StatusCode)
	}
}

func TestAuthTokenInvalid(t *testing.T) {
	_, ts := newTestServerWithConfig(t, config.SignalConfig{
		DevMode:   true,
		AuthToken: "secret-token-123",
	})

	// Connection with wrong token via query param
	u := wsURL(ts) + "?token=wrong-token"
	_, resp, err := (&websocket.Dialer{}).Dial(u, nil)
	if err == nil {
		t.Fatal("expected auth rejection for wrong token")
	}
	if resp != nil && resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", resp.StatusCode)
	}
}

func TestAuthTokenValidViaQuery(t *testing.T) {
	_, ts := newTestServerWithConfig(t, config.SignalConfig{
		DevMode:   true,
		AuthToken: "secret-token-123",
	})

	u := wsURL(ts) + "?token=secret-token-123"
	conn := dialWS(t, u)
	defer conn.Close()

	// Should be able to register
	reg := protocol.NewMessage(protocol.MsgRegister, protocol.RegisterPayload{
		Name:     "auth-test-host",
		Platform: "linux",
	})
	conn.WriteJSON(reg)

	var resp protocol.Message
	conn.ReadJSON(&resp)
	if resp.Type != protocol.MsgRegister {
		t.Errorf("expected register response, got %s", resp.Type)
	}
}

func TestAuthTokenValidViaBearer(t *testing.T) {
	_, ts := newTestServerWithConfig(t, config.SignalConfig{
		DevMode:   true,
		AuthToken: "secret-token-123",
	})

	u := wsURL(ts)
	conn, _, err := (&websocket.Dialer{}).Dial(u, http.Header{
		"Authorization": []string{"Bearer secret-token-123"},
	})
	if err != nil {
		t.Fatalf("dial with bearer token: %v", err)
	}
	defer conn.Close()

	// Should work
	reg := protocol.NewMessage(protocol.MsgRegister, protocol.RegisterPayload{
		Name:     "bearer-host",
		Platform: "linux",
	})
	conn.WriteJSON(reg)

	var resp protocol.Message
	conn.ReadJSON(&resp)
	if resp.Type != protocol.MsgRegister {
		t.Errorf("expected register response, got %s", resp.Type)
	}
}

func TestAuthTokenDisabled(t *testing.T) {
	// No auth token = open access
	_, ts := newTestServer(t)
	conn := dialWS(t, wsURL(ts))
	defer conn.Close()

	reg := protocol.NewMessage(protocol.MsgRegister, protocol.RegisterPayload{
		Name:     "open-host",
		Platform: "linux",
	})
	conn.WriteJSON(reg)

	var resp protocol.Message
	conn.ReadJSON(&resp)
	if resp.Type != protocol.MsgRegister {
		t.Errorf("expected register response, got %s", resp.Type)
	}
}

func TestRegisterEmptyName(t *testing.T) {
	_, ts := newTestServer(t)
	conn := dialWS(t, wsURL(ts))
	defer conn.Close()

	conn.WriteJSON(protocol.NewMessage(protocol.MsgRegister, protocol.RegisterPayload{
		Name: "", Platform: "linux", Arch: "arm64",
	}))

	var resp protocol.Message
	if err := conn.ReadJSON(&resp); err != nil {
		t.Fatal(err)
	}
	if resp.Type != protocol.MsgError {
		t.Errorf("expected error for empty name, got %s", resp.Type)
	}
}

func TestRegisterLongName(t *testing.T) {
	_, ts := newTestServer(t)
	conn := dialWS(t, wsURL(ts))
	defer conn.Close()

	longName := strings.Repeat("x", 100)
	conn.WriteJSON(protocol.NewMessage(protocol.MsgRegister, protocol.RegisterPayload{
		Name: longName, Platform: "linux", Arch: "arm64",
	}))

	var resp protocol.Message
	if err := conn.ReadJSON(&resp); err != nil {
		t.Fatal(err)
	}
	if resp.Type != protocol.MsgError {
		t.Errorf("expected error for long name, got %s", resp.Type)
	}
}

func TestRegisterWhitespaceName(t *testing.T) {
	_, ts := newTestServer(t)
	conn := dialWS(t, wsURL(ts))
	defer conn.Close()

	conn.WriteJSON(protocol.NewMessage(protocol.MsgRegister, protocol.RegisterPayload{
		Name: "   ", Platform: "linux", Arch: "arm64",
	}))

	var resp protocol.Message
	if err := conn.ReadJSON(&resp); err != nil {
		t.Fatal(err)
	}
	if resp.Type != protocol.MsgError {
		t.Errorf("expected error for whitespace name, got %s", resp.Type)
	}
}

func TestRegisterDuplicateState(t *testing.T) {
	_, ts := newTestServer(t)
	conn := dialWS(t, wsURL(ts))
	defer conn.Close()

	// First registration should succeed
	conn.WriteJSON(protocol.NewMessage(protocol.MsgRegister, protocol.RegisterPayload{
		Name: "test-host", Platform: "linux", Arch: "arm64",
	}))
	var resp1 protocol.Message
	conn.ReadJSON(&resp1)
	if resp1.Type != protocol.MsgRegister {
		t.Fatalf("first register should succeed, got %s", resp1.Type)
	}

	// Second registration should fail
	conn.WriteJSON(protocol.NewMessage(protocol.MsgRegister, protocol.RegisterPayload{
		Name: "test-host-2", Platform: "linux", Arch: "arm64",
	}))
	var resp2 protocol.Message
	conn.ReadJSON(&resp2)
	if resp2.Type != protocol.MsgError {
		t.Errorf("duplicate register should error, got %s", resp2.Type)
	}
}

func TestConnectNonExistentHost(t *testing.T) {
	_, ts := newTestServer(t)
	conn := dialWS(t, wsURL(ts))
	defer conn.Close()

	conn.WriteJSON(protocol.NewMessage(protocol.MsgConnect, protocol.ConnectPayload{
		HostID: "non-existent-id",
	}))

	var resp protocol.Message
	if err := conn.ReadJSON(&resp); err != nil {
		t.Fatal(err)
	}
	if resp.Type != protocol.MsgError {
		t.Errorf("expected error for non-existent host, got %s", resp.Type)
	}
}

func TestRelayWithoutRoom(t *testing.T) {
	_, ts := newTestServer(t)
	conn := dialWS(t, wsURL(ts))
	defer conn.Close()

	// Register but don't join a room
	conn.WriteJSON(protocol.NewMessage(protocol.MsgRegister, protocol.RegisterPayload{
		Name: "solo-host", Platform: "linux", Arch: "arm64",
	}))
	var regResp protocol.Message
	conn.ReadJSON(&regResp)

	// Try to relay an offer without being in a room
	conn.WriteJSON(protocol.Message{
		Type:    protocol.MsgOffer,
		Payload: toRaw(map[string]string{"sdp": "test"}),
	})

	var resp protocol.Message
	if err := conn.ReadJSON(&resp); err != nil {
		t.Fatal(err)
	}
	if resp.Type != protocol.MsgError {
		t.Errorf("expected error for relay without room, got %s", resp.Type)
	}
}

// ======== Room Lifecycle Tests ========

func TestRoomLifecycle(t *testing.T) {
	s, ts := newTestServer(t)

	// Register host
	hostConn := dialWS(t, wsURL(ts))
	defer hostConn.Close()
	hostConn.WriteJSON(protocol.NewMessage(protocol.MsgRegister, protocol.RegisterPayload{
		Name: "lifecycle-host", Platform: "linux",
	}))
	var hostResp protocol.Message
	hostConn.ReadJSON(&hostResp)
	hostID := ""
	json.Unmarshal(hostResp.Payload, &struct {
		ID *string `json:"id"`
	}{ID: &hostID})

	// Connect client
	clientConn := dialWS(t, wsURL(ts))
	defer clientConn.Close()
	clientConn.WriteJSON(protocol.NewMessage(protocol.MsgConnect, protocol.ConnectPayload{
		HostID: hostID,
	}))
	var connectResp protocol.Message
	clientConn.ReadJSON(&connectResp)
	if connectResp.Type != protocol.MsgRoomReady {
		t.Fatalf("expected room_ready, got %s", connectResp.Type)
	}

	// Host should receive connect notification
	var hostNotify protocol.Message
	hostConn.ReadJSON(&hostNotify)
	if hostNotify.Type != protocol.MsgConnect {
		t.Errorf("host should get connect notification, got %s", hostNotify.Type)
	}

	// Verify stats
	s.mu.RLock()
	rooms := len(s.rooms)
	peers := len(s.peers)
	s.mu.RUnlock()
	if rooms != 1 {
		t.Errorf("rooms = %d, want 1", rooms)
	}
	if peers != 2 {
		t.Errorf("peers = %d, want 2", peers)
	}
}

func TestPeerDisconnectRoomCleanup(t *testing.T) {
	s, ts := newTestServer(t)

	// Register host + connect client
	hostConn := dialWS(t, wsURL(ts))
	hostConn.WriteJSON(protocol.NewMessage(protocol.MsgRegister, protocol.RegisterPayload{
		Name: "disconnect-host", Platform: "linux",
	}))
	var hostResp protocol.Message
	hostConn.ReadJSON(&hostResp)
	var regData struct {
		ID string `json:"id"`
	}
	json.Unmarshal(hostResp.Payload, &regData)

	clientConn := dialWS(t, wsURL(ts))
	clientConn.WriteJSON(protocol.NewMessage(protocol.MsgConnect, protocol.ConnectPayload{
		HostID: regData.ID,
	}))
	var connectResp protocol.Message
	clientConn.ReadJSON(&connectResp) // room_ready

	var hostNotify protocol.Message
	hostConn.ReadJSON(&hostNotify) // connect notification

	// Client disconnects
	clientConn.Close()

	// Host should receive peer_left notification
	var leftMsg protocol.Message
	hostConn.ReadJSON(&leftMsg)
	if leftMsg.Type != protocol.MsgPeerLeft {
		t.Errorf("expected peer_left, got %s", leftMsg.Type)
	}

	// Room should be cleaned up
	s.mu.RLock()
	rooms := len(s.rooms)
	s.mu.RUnlock()
	if rooms != 0 {
		t.Errorf("rooms after disconnect = %d, want 0", rooms)
	}
}

func TestConcurrentRegistration(t *testing.T) {
	_, ts := newTestServer(t)

	const numHosts = 10
	results := make(chan error, numHosts)

	for i := 0; i < numHosts; i++ {
		go func(idx int) {
			conn, _, err := (&websocket.Dialer{}).Dial(wsURL(ts), nil)
			if err != nil {
				results <- err
				return
			}
			defer conn.Close()

			conn.WriteJSON(protocol.NewMessage(protocol.MsgRegister, protocol.RegisterPayload{
				Name: fmt.Sprintf("concurrent-host-%d", idx),
				Platform: "linux",
			}))

			var resp protocol.Message
			if err := conn.ReadJSON(&resp); err != nil {
				results <- err
				return
			}
			if resp.Type != protocol.MsgRegister {
				results <- fmt.Errorf("host %d: expected register, got %s", idx, resp.Type)
				return
			}
			results <- nil
		}(i)
	}

	for i := 0; i < numHosts; i++ {
		if err := <-results; err != nil {
			t.Errorf("host %d failed: %v", i, err)
		}
	}
}

func TestConnectToOfflineHost(t *testing.T) {
	_, ts := newTestServer(t)

	clientConn := dialWS(t, wsURL(ts))
	defer clientConn.Close()

	clientConn.WriteJSON(protocol.NewMessage(protocol.MsgConnect, protocol.ConnectPayload{
		HostID: "nonexistent-host-id",
	}))

	var resp protocol.Message
	clientConn.ReadJSON(&resp)
	if resp.Type != protocol.MsgError {
		t.Errorf("expected error for offline host, got %s", resp.Type)
	}
}

func TestDuplicateRegistration(t *testing.T) {
	_, ts := newTestServer(t)

	conn := dialWS(t, wsURL(ts))
	defer conn.Close()

	// First registration should succeed
	conn.WriteJSON(protocol.NewMessage(protocol.MsgRegister, protocol.RegisterPayload{
		Name: "dup-host", Platform: "linux",
	}))
	var resp1 protocol.Message
	conn.ReadJSON(&resp1)
	if resp1.Type != protocol.MsgRegister {
		t.Errorf("first registration should succeed, got %s", resp1.Type)
	}

	// Second registration should fail
	conn.WriteJSON(protocol.NewMessage(protocol.MsgRegister, protocol.RegisterPayload{
		Name: "dup-host-2", Platform: "linux",
	}))
	var resp2 protocol.Message
	conn.ReadJSON(&resp2)
	if resp2.Type != protocol.MsgError {
		t.Errorf("duplicate registration should error, got %s", resp2.Type)
	}
}
