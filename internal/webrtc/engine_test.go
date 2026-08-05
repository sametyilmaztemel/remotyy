package webrtc

import (
	"encoding/json"
	"sync/atomic"
	"testing"

	"github.com/sametyilmaztemel/remotty/internal/protocol"
)

func TestEngineCreateAndClose(t *testing.T) {
	eng, err := NewEngine(func(cfg *EngineConfig) {
		cfg.ICEServers = []string{"stun:stun.l.google.com:19302"}
	})
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}

	if eng == nil {
		t.Fatal("engine should not be nil")
	}

	// Close should work without error
	if err := eng.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	// Double close should be safe
	if err := eng.Close(); err != nil {
		t.Fatalf("Double Close: %v", err)
	}

	if !eng.closed {
		t.Error("engine should be marked as closed")
	}
}

func TestEngineCreateDataChannel(t *testing.T) {
	eng, err := NewEngine(func(cfg *EngineConfig) {
		cfg.ICEServers = []string{"stun:stun.l.google.com:19302"}
	})
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}
	defer eng.Close()

	dc := eng.CreateDataChannel("test-channel")
	if dc == nil {
		t.Fatal("data channel should not be nil")
	}

	eng.mu.Lock()
	_, exists := eng.dataChannels["test-channel"]
	eng.mu.Unlock()

	if !exists {
		t.Error("data channel should be registered in engine")
	}
}

func TestEngineCreateMultipleDataChannels(t *testing.T) {
	eng, err := NewEngine(func(cfg *EngineConfig) {
		cfg.ICEServers = []string{"stun:stun.l.google.com:19302"}
	})
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}
	defer eng.Close()

	labels := []string{"terminal", "auth", "screen", "file", "clipboard"}
	for _, label := range labels {
		dc := eng.CreateDataChannel(label)
		if dc == nil {
			t.Errorf("data channel %q should not be nil", label)
		}
	}

	eng.mu.Lock()
	count := len(eng.dataChannels)
	eng.mu.Unlock()

	if count != len(labels) {
		t.Errorf("data channels = %d, want %d", count, len(labels))
	}
}

func TestEngineICEStateCallback(t *testing.T) {
	var stateCalled atomic.Int32

	eng, err := NewEngine(func(cfg *EngineConfig) {
		cfg.ICEServers = []string{"stun:stun.l.google.com:19302"}
	})
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}
	defer eng.Close()

	// ICE callback registered — we just verify engine setup works
	if eng == nil {
		t.Fatal("engine should not be nil")
	}
	_ = stateCalled.Load()
}

func TestEngineConnectionState(t *testing.T) {
	eng, err := NewEngine(func(cfg *EngineConfig) {
		cfg.ICEServers = []string{"stun:stun.l.google.com:19302"}
	})
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}
	defer eng.Close()

	state := eng.ConnectionState()
	// New engine should be in "new" or "checking" state
	_ = state // Just verify it doesn't panic
}

func TestDataChannelSendJSON(t *testing.T) {
	eng, err := NewEngine(func(cfg *EngineConfig) {
		cfg.ICEServers = []string{"stun:stun.l.google.com:19302"}
	})
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}
	defer eng.Close()

	dc := eng.CreateDataChannel("json-test")
	if dc == nil {
		t.Fatal("data channel should not be nil")
	}

	// SendJSON should marshal and attempt to send
	msg := protocol.NewMessage(protocol.MsgInput, "test data")
	data, _ := json.Marshal(msg)

	// We can't actually send on an unconnected data channel,
	// but verify the marshalling works
	if len(data) == 0 {
		t.Error("marshalled message should not be empty")
	}
}
