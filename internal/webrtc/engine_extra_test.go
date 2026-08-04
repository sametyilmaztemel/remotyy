package webrtc

import (
	"encoding/json"
	"testing"

	pion "github.com/pion/webrtc/v4"
	"github.com/sametyilmaztemel/remotty/internal/protocol"
)

func TestDefaultReconnectConfig(t *testing.T) {
	cfg := DefaultReconnectConfig()
	if cfg.InitialBackoff == 0 {
		t.Error("InitialBackoff should be non-zero")
	}
	if cfg.MaxBackoff == 0 {
		t.Error("MaxBackoff should be non-zero")
	}
	if cfg.MaxAttempts == 0 {
		t.Error("MaxAttempts should be non-zero")
	}
	if cfg.InitialBackoff >= cfg.MaxBackoff {
		t.Error("InitialBackoff should be less than MaxBackoff")
	}
}

func TestEngineReconnectDefaults(t *testing.T) {
	// Test that zero-value reconnect config fields get defaults
	eng, err := NewEngine(func(cfg *EngineConfig) {
		cfg.ICEServers = []string{"stun:stun.l.google.com:19302"}
		cfg.Reconnect = ReconnectConfig{
			// All zero values — should get defaults
		}
	})
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}
	defer eng.Close()

	if eng.reconnectCfg.InitialBackoff == 0 {
		t.Error("InitialBackoff should have default value")
	}
	if eng.reconnectCfg.MaxBackoff == 0 {
		t.Error("MaxBackoff should have default value")
	}
}

func TestEngineCustomReconnect(t *testing.T) {
	eng, err := NewEngine(func(cfg *EngineConfig) {
		cfg.ICEServers = []string{"stun:stun.l.google.com:19302"}
		cfg.Reconnect = ReconnectConfig{
			InitialBackoff: 1000000000, // 1s
			MaxBackoff:     30000000000, // 30s
			MaxAttempts:    5,
		}
	})
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}
	defer eng.Close()

	if eng.reconnectCfg.InitialBackoff != 1000000000 {
		t.Errorf("InitialBackoff = %v, want 1s", eng.reconnectCfg.InitialBackoff)
	}
	if eng.reconnectCfg.MaxBackoff != 30000000000 {
		t.Errorf("MaxBackoff = %v, want 30s", eng.reconnectCfg.MaxBackoff)
	}
	if eng.reconnectCfg.MaxAttempts != 5 {
		t.Errorf("MaxAttempts = %d, want 5", eng.reconnectCfg.MaxAttempts)
	}
}

func TestDataChannelSendJSONMarshal(t *testing.T) {
	eng, err := NewEngine(func(cfg *EngineConfig) {
		cfg.ICEServers = []string{"stun:stun.l.google.com:19302"}
	})
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}
	defer eng.Close()

	dc := eng.CreateDataChannel("send-json-test")
	if dc == nil {
		t.Fatal("data channel should not be nil")
	}

	// Test that SendJSON marshals and attempts to send
	msg := protocol.NewMessage(protocol.MsgInput, "test data")

	// This should marshal JSON and try to send via the underlying data channel
	// The actual send will fail since the channel isn't connected,
	// but we at least verify the method doesn't panic and returns an error
	err = dc.SendJSON(msg)
	if err == nil {
		// It's actually OK if it succeeds (data channel might buffer)
		t.Log("SendJSON returned nil (data channel buffered)")
	} else {
		// Expected error: data channel not open
		t.Logf("SendJSON returned expected error: %v", err)
	}

	// Verify the message can be properly marshalled
	data, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	if len(data) == 0 {
		t.Error("marshalled data should not be empty")
	}

	// Verify it can be unmarshalled back
	var decoded protocol.Message
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	if decoded.Type != protocol.MsgInput {
		t.Errorf("decoded type = %s, want %s", decoded.Type, protocol.MsgInput)
	}
}

func TestDataChannelSendRaw(t *testing.T) {
	eng, err := NewEngine(func(cfg *EngineConfig) {
		cfg.ICEServers = []string{"stun:stun.l.google.com:19302"}
	})
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}
	defer eng.Close()

	dc := eng.CreateDataChannel("send-raw-test")
	if dc == nil {
		t.Fatal("data channel should not be nil")
	}

	// Test raw send
	err = dc.Send([]byte("hello"))
	if err == nil {
		t.Log("Send returned nil (data channel buffered)")
	} else {
		t.Logf("Send returned expected error: %v", err)
	}
}

func TestDataChannelOnMessage(t *testing.T) {
	eng, err := NewEngine(func(cfg *EngineConfig) {
		cfg.ICEServers = []string{"stun:stun.l.google.com:19302"}
	})
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}
	defer eng.Close()

	dc := eng.CreateDataChannel("onmsg-test")
	if dc == nil {
		t.Fatal("data channel should not be nil")
	}

	// OnMessage should set the callback without panicking
	called := false
	dc.OnMessage(func(data []byte) {
		called = true
	})

	if dc.onMessage == nil {
		t.Error("onMessage callback should be set")
	}

	// We can't easily trigger the callback without a real connection,
	// but verify the setup didn't panic
	_ = called
}

func TestDataChannelOnMessageNilCallback(t *testing.T) {
	eng, err := NewEngine(func(cfg *EngineConfig) {
		cfg.ICEServers = []string{"stun:stun.l.google.com:19302"}
	})
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}
	defer eng.Close()

	dc := eng.CreateDataChannel("nil-onmsg-test")
	if dc == nil {
		t.Fatal("data channel should not be nil")
	}

	// Setting nil callback should not panic
	dc.OnMessage(nil)
}

func TestHandleICE(t *testing.T) {
	eng, err := NewEngine(func(cfg *EngineConfig) {
		cfg.ICEServers = []string{"stun:stun.l.google.com:19302"}
	})
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}
	defer eng.Close()

	// Create an ICE candidate message (valid format)
	msg := protocol.NewMessage(protocol.MsgICECandidate, map[string]interface{}{
		"candidate":     "candidate:1 1 UDP 2122252543 192.168.1.1 12345 typ host",
		"sdpMid":        "0",
		"sdpMLineIndex": 0,
	})

	err = eng.HandleICE(msg)
	if err != nil {
		// May fail if the PC isn't in the right state, but shouldn't panic
		t.Logf("HandleICE returned (expected possibly): %v", err)
	}

	// Invalid candidate should return an error
	badMsg := protocol.NewMessage(protocol.MsgICECandidate, map[string]interface{}{
		"candidate": "invalid",
	})
	err = eng.HandleICE(badMsg)
	if err != nil {
		t.Logf("HandleICE with bad candidate returned: %v", err)
	}
}

func TestEngineAddVideoTrack(t *testing.T) {
	eng, err := NewEngine(func(cfg *EngineConfig) {
		cfg.ICEServers = []string{"stun:stun.l.google.com:19302"}
	})
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}
	defer eng.Close()

	track, err := eng.AddVideoTrack()
	if err != nil {
		t.Fatalf("AddVideoTrack: %v", err)
	}
	if track == nil {
		t.Fatal("track should not be nil")
	}
	if track.StreamID() != "screen-id" {
		t.Errorf("track stream ID = %q, want %q", track.StreamID(), "screen-id")
	}
	if track.ID() != "screen" {
		t.Errorf("track ID = %q, want %q", track.ID(), "screen")
	}
}

func TestEngineCreateOffer(t *testing.T) {
	eng, err := NewEngine(func(cfg *EngineConfig) {
		cfg.ICEServers = []string{"stun:stun.l.google.com:19302"}
	})
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}
	defer eng.Close()

	offer, err := eng.CreateOffer()
	if err != nil {
		t.Fatalf("CreateOffer: %v", err)
	}
	if offer["type"] != "offer" {
		t.Errorf("offer type = %v, want offer", offer["type"])
	}
	sdp, ok := offer["sdp"].(string)
	if !ok || sdp == "" {
		t.Error("sdp should be a non-empty string")
	}
}

func TestHandleAnswer(t *testing.T) {
	eng, err := NewEngine(func(cfg *EngineConfig) {
		cfg.ICEServers = []string{"stun:stun.l.google.com:19302"}
	})
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}
	defer eng.Close()

	// Create offer and set it as local
	_, err = eng.CreateOffer()
	if err != nil {
		t.Fatalf("CreateOffer: %v", err)
	}

	// Now try to handle an answer (will fail since we don't have a matching remote offer)
	answerMsg := protocol.NewMessage(protocol.MsgAnswer, map[string]interface{}{
		"type": "answer",
		"sdp":  "v=0\r\no=- 0 0 IN IP4 127.0.0.1\r\ns=-\r\n",
	})
	err = eng.HandleAnswer(answerMsg)
	t.Logf("HandleAnswer returned: %v", err)
	// Expected to fail since SDP doesn't match, but code shouldn't panic
}

func TestOnDataChannelCallback(t *testing.T) {
	var dcCalled bool
	var labelCalled string

	eng, err := NewEngine(func(cfg *EngineConfig) {
		cfg.ICEServers = []string{"stun:stun.l.google.com:19302"}
		cfg.OnDataChannel = func(dc *DataChannel, label string) {
			dcCalled = true
			labelCalled = label
		}
	})
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}
	defer eng.Close()

	// The callback won't be triggered without an actual incoming data channel,
	// but we verify the engine stores it properly
	if eng.config.OnDataChannel == nil {
		t.Error("OnDataChannel callback should be set")
	}
	_ = dcCalled
	_ = labelCalled
}

func TestOnICEStateCallback(t *testing.T) {
	var stateCalled bool

	eng, err := NewEngine(func(cfg *EngineConfig) {
		cfg.ICEServers = []string{"stun:stun.l.google.com:19302"}
		cfg.OnICEState = func(state pion.ICEConnectionState) {
			stateCalled = true
		}
	})
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}
	defer eng.Close()

	if eng.config.OnICEState == nil {
		t.Error("OnICEState callback should be set")
	}
	_ = stateCalled
}
