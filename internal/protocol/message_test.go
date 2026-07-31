package protocol

import (
	"encoding/json"
	"testing"
)

func TestNewMessage(t *testing.T) {
	payload := map[string]string{"key": "value"}
	msg := NewMessage(MsgRegister, payload)

	if msg.Type != MsgRegister {
		t.Errorf("expected register type, got %s", msg.Type)
	}

	if len(msg.Payload) == 0 {
		t.Error("expected non-empty payload")
	}

	// Verify payload round-trips
	var decoded map[string]string
	if err := json.Unmarshal(msg.Payload, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded["key"] != "value" {
		t.Errorf("expected value, got %s", decoded["key"])
	}
}

func TestRegisterPayload(t *testing.T) {
	p := RegisterPayload{
		Name:     "test",
		Platform: "linux",
		Arch:     "arm64",
		Version:  "0.2.0",
		Features: []string{"terminal", "screen"},
	}

	data, err := json.Marshal(p)
	if err != nil {
		t.Fatal(err)
	}

	var decoded RegisterPayload
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}

	if decoded.Name != "test" {
		t.Errorf("expected test, got %s", decoded.Name)
	}
	if len(decoded.Features) != 2 {
		t.Errorf("expected 2 features, got %d", len(decoded.Features))
	}
}

func TestMessageTypes(t *testing.T) {
	tests := []struct {
		msgType MessageType
		want    string
	}{
		{MsgRegister, "register"},
		{MsgOffer, "offer"},
		{MsgAuthOK, "auth_ok"},
		{MsgFileChunk, "file_chunk"},
		{MsgFileComplete, "file_complete"},
		{MsgFileError, "file_error"},
		{MsgClipboardData, "clipboard_data"},
		{MsgClipboardRequest, "clipboard_request"},
	}

	for _, tt := range tests {
		if string(tt.msgType) != tt.want {
			t.Errorf("MsgType %s: expected %s", tt.msgType, tt.want)
		}
	}
}
