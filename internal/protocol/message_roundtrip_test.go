package protocol

import (
	"encoding/json"
	"testing"
)

func toRaw(v interface{}) json.RawMessage {
	data, _ := json.Marshal(v)
	return data
}

func TestMessageFullRoundtrip(t *testing.T) {
	original := Message{
		Type: MsgInput,
		Payload: toRaw("hello terminal"),
		From: "peer-1",
		To:   "peer-2",
		Room: "room-abc",
		ID:   "msg-123",
		Time: 1717500000,
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var decoded Message
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if decoded.Type != original.Type {
		t.Errorf("Type: got %q, want %q", decoded.Type, original.Type)
	}
	if decoded.From != original.From {
		t.Errorf("From: got %q, want %q", decoded.From, original.From)
	}
	if decoded.To != original.To {
		t.Errorf("To: got %q, want %q", decoded.To, original.To)
	}
	if decoded.Room != original.Room {
		t.Errorf("Room: got %q, want %q", decoded.Room, original.Room)
	}
	if decoded.ID != original.ID {
		t.Errorf("ID: got %q, want %q", decoded.ID, original.ID)
	}
	if decoded.Time != original.Time {
		t.Errorf("Time: got %d, want %d", decoded.Time, original.Time)
	}
}

func TestMessageEmptyPayload(t *testing.T) {
	msg := Message{Type: MsgHeartbeat}
	data, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	// Payload should be omitted when nil/empty
	var raw map[string]interface{}
	json.Unmarshal(data, &raw)
	if _, ok := raw["payload"]; ok {
		t.Error("payload should be omitted for nil payload")
	}
}

func TestMessageNilPayloadUnmarshal(t *testing.T) {
	raw := `{"type":"heartbeat"}`
	var msg Message
	if err := json.Unmarshal([]byte(raw), &msg); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if msg.Type != MsgHeartbeat {
		t.Errorf("Type: got %q, want %q", msg.Type, MsgHeartbeat)
	}
	if msg.Payload != nil {
		t.Error("Payload should be nil for missing field")
	}
}

func TestNewMessageFromPayload(t *testing.T) {
	payload := ResizePayload{Rows: 40, Cols: 120}
	msg := NewMessage(MsgResize, payload)

	if msg.Type != MsgResize {
		t.Errorf("Type: got %q, want %q", msg.Type, MsgResize)
	}

	var decoded ResizePayload
	if err := json.Unmarshal(msg.Payload, &decoded); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}
	if decoded.Rows != 40 || decoded.Cols != 120 {
		t.Errorf("Payload: got Rows=%d Cols=%d, want 40/120", decoded.Rows, decoded.Cols)
	}
}

func TestNewMessageNilPayload(t *testing.T) {
	msg := NewMessage(MsgPing, nil)
	if msg.Type != MsgPing {
		t.Errorf("Type: got %q, want %q", msg.Type, MsgPing)
	}
	// json.Marshal(nil) produces "null"
	if string(msg.Payload) != "null" {
		t.Errorf("Payload: got %q, want \"null\"", string(msg.Payload))
	}
}

func TestMessageTypeConstants(t *testing.T) {
	types := []struct {
		val    MessageType
		expect string
	}{
		{MsgRegister, "register"},
		{MsgHeartbeat, "heartbeat"},
		{MsgListHosts, "list_hosts"},
		{MsgConnect, "connect"},
		{MsgOffer, "offer"},
		{MsgAnswer, "answer"},
		{MsgICECandidate, "ice_candidate"},
		{MsgAuth, "auth"},
		{MsgAuthOK, "auth_ok"},
		{MsgAuthFail, "auth_fail"},
		{MsgInput, "input"},
		{MsgOutput, "output"},
		{MsgResize, "resize"},
		{MsgScreenStart, "screen_start"},
		{MsgScreenFrame, "screen_frame"},
		{MsgScreenStop, "screen_stop"},
		{MsgMouseMove, "mouse_move"},
		{MsgMouseClick, "mouse_click"},
		{MsgMouseScroll, "mouse_scroll"},
		{MsgKeyPress, "key_press"},
		{MsgFileRequest, "file_request"},
		{MsgFileChunk, "file_chunk"},
		{MsgFileComplete, "file_complete"},
		{MsgFileError, "file_error"},
		{MsgClipboard, "clipboard"},
		{MsgClipboardData, "clipboard_data"},
		{MsgClipboardRequest, "clipboard_request"},
		{MsgPing, "ping"},
		{MsgPong, "pong"},
		{MsgError, "error"},
	}

	for _, tc := range types {
		if string(tc.val) != tc.expect {
			t.Errorf("MessageType constant: got %q, want %q", tc.val, tc.expect)
		}
	}
}

func TestRegisterPayloadRoundtrip(t *testing.T) {
	original := RegisterPayload{
		Name:     "my-host",
		Platform: "linux",
		Arch:     "arm64",
		Version:  "0.7.1",
		Features: []string{"terminal", "screen", "file"},
		DeviceID: "dev-abc123",
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var decoded RegisterPayload
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if decoded.Name != original.Name {
		t.Errorf("Name: got %q, want %q", decoded.Name, original.Name)
	}
	if len(decoded.Features) != len(original.Features) {
		t.Fatalf("Features length: got %d, want %d", len(decoded.Features), len(original.Features))
	}
	for i, f := range decoded.Features {
		if f != original.Features[i] {
			t.Errorf("Features[%d]: got %q, want %q", i, f, original.Features[i])
		}
	}
}

func TestRegisterPayloadEmptyFeatures(t *testing.T) {
	original := RegisterPayload{
		Name:     "minimal-host",
		Platform: "darwin",
		Arch:     "amd64",
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var decoded RegisterPayload
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	// nil slice marshals to null, unmarshals to nil
	if decoded.Features != nil {
		t.Errorf("Features should be nil for empty, got %v", decoded.Features)
	}
}

func TestHostInfoOmitEmpty(t *testing.T) {
	h := HostInfo{
		ID:       "h-1",
		Name:     "test",
		Platform: "linux",
		Online:   true,
	}

	data, err := json.Marshal(h)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var raw map[string]interface{}
	json.Unmarshal(data, &raw)

	// DeviceID and Ping should be omitted (omitempty, zero values)
	if _, ok := raw["device_id"]; ok {
		t.Error("device_id should be omitted when empty")
	}
	if _, ok := raw["ping"]; ok {
		t.Error("ping should be omitted when zero")
	}
}

func TestFileChunkPayloadRoundtrip(t *testing.T) {
	chunkData := []byte{0x01, 0x02, 0x03, 0x04, 0x05}
	original := FileChunkPayload{
		TransferID: "tf-789",
		Index:      42,
		Data:       chunkData,
		Checksum:   "abc123def456",
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var decoded FileChunkPayload
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if decoded.TransferID != original.TransferID {
		t.Errorf("TransferID: got %q, want %q", decoded.TransferID, original.TransferID)
	}
	if decoded.Index != original.Index {
		t.Errorf("Index: got %d, want %d", decoded.Index, original.Index)
	}
	if len(decoded.Data) != len(original.Data) {
		t.Fatalf("Data length: got %d, want %d", len(decoded.Data), len(original.Data))
	}
	for i, b := range decoded.Data {
		if b != original.Data[i] {
			t.Errorf("Data[%d]: got %02x, want %02x", i, b, original.Data[i])
		}
	}
}

func TestErrorPayloadRoundtrip(t *testing.T) {
	original := ErrorPayload{
		Code:    403,
		Message: "not authorized",
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var decoded ErrorPayload
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if decoded.Code != original.Code || decoded.Message != original.Message {
		t.Errorf("got Code=%d Message=%q, want Code=%d Message=%q",
			decoded.Code, decoded.Message, original.Code, original.Message)
	}
}

func TestScreenConfigPayloadDefaults(t *testing.T) {
	raw := `{"fps":30,"quality":80}`
	var cfg ScreenConfigPayload
	if err := json.Unmarshal([]byte(raw), &cfg); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if cfg.FPS != 30 {
		t.Errorf("FPS: got %d, want 30", cfg.FPS)
	}
	if cfg.Quality != 80 {
		t.Errorf("Quality: got %d, want 80", cfg.Quality)
	}
	// MaxDimension and CaptureCursor should be zero/false
	if cfg.MaxDimension != 0 {
		t.Errorf("MaxDimension: got %d, want 0", cfg.MaxDimension)
	}
	if cfg.CaptureCursor {
		t.Error("CaptureCursor should be false by default")
	}
}

func TestMouseMovePayloadNegative(t *testing.T) {
	raw := `{"x":-1.5,"y":-100.0}`
	var payload MouseMovePayload
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if payload.X != -1.5 {
		t.Errorf("X: got %f, want -1.5", payload.X)
	}
	if payload.Y != -100.0 {
		t.Errorf("Y: got %f, want -100.0", payload.Y)
	}
}

func TestKeyPayloadCharsOptional(t *testing.T) {
	// KeyCode only
	raw1 := `{"key_code":65}`
	var p1 KeyPayload
	json.Unmarshal([]byte(raw1), &p1)
	if p1.KeyCode != 65 || p1.Chars != "" {
		t.Errorf("KeyCode only: got KeyCode=%d Chars=%q", p1.KeyCode, p1.Chars)
	}

	// Chars only
	raw2 := `{"chars":"hello"}`
	var p2 KeyPayload
	json.Unmarshal([]byte(raw2), &p2)
	if p2.KeyCode != 0 || p2.Chars != "hello" {
		t.Errorf("Chars only: got KeyCode=%d Chars=%q", p2.KeyCode, p2.Chars)
	}

	// Both
	raw3 := `{"key_code":13,"chars":"\n"}`
	var p3 KeyPayload
	json.Unmarshal([]byte(raw3), &p3)
	if p3.KeyCode != 13 || p3.Chars != "\n" {
		t.Errorf("Both: got KeyCode=%d Chars=%q", p3.KeyCode, p3.Chars)
	}
}

func TestClipboardPayloadRoundtrip(t *testing.T) {
	original := ClipboardPayload{Text: "hello world 🌍"}
	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var decoded ClipboardPayload
	json.Unmarshal(data, &decoded)
	if decoded.Text != original.Text {
		t.Errorf("Text: got %q, want %q", decoded.Text, original.Text)
	}
}
func TestClipboardDataRoundtrip(t *testing.T) {
	original := ClipboardData{ClipboardText: "clip data here", Timestamp: 1717500123}
	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var decoded ClipboardData
	json.Unmarshal(data, &decoded)
	if decoded.ClipboardText != original.ClipboardText {
		t.Errorf("ClipboardText: got %q, want %q", decoded.ClipboardText, original.ClipboardText)
	}
	if decoded.Timestamp != original.Timestamp {
		t.Errorf("Timestamp: got %d, want %d", decoded.Timestamp, original.Timestamp)
	}
}

func TestClipboardRequestRoundtrip(t *testing.T) {
	original := ClipboardRequest{RequestID: "req-001"}
	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var decoded ClipboardRequest
	json.Unmarshal(data, &decoded)
	if decoded.RequestID != original.RequestID {
		t.Errorf("RequestID: got %q, want %q", decoded.RequestID, original.RequestID)
	}
}

func TestClipboardDataEmptyText(t *testing.T) {
	original := ClipboardData{Timestamp: 1717500999}
	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var decoded ClipboardData
	json.Unmarshal(data, &decoded)
	if decoded.ClipboardText != "" {
		t.Errorf("ClipboardText: got %q, want empty", decoded.ClipboardText)
	}
	if decoded.Timestamp != 1717500999 {
		t.Errorf("Timestamp: got %d, want 1717500999", decoded.Timestamp)
	}
}


func TestFileProgressPayloadRoundtrip(t *testing.T) {
	original := FileProgressPayload{
		TransferID: "tf-555",
		BytesSent:  1024 * 1024 * 50,
		TotalBytes: 1024 * 1024 * 100,
		Speed:      5242880,
	}
	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var decoded FileProgressPayload
	json.Unmarshal(data, &decoded)
	if decoded.TransferID != original.TransferID {
		t.Errorf("TransferID: got %q, want %q", decoded.TransferID, original.TransferID)
	}
	if decoded.BytesSent != original.BytesSent {
		t.Errorf("BytesSent: got %d, want %d", decoded.BytesSent, original.BytesSent)
	}
	if decoded.Speed != original.Speed {
		t.Errorf("Speed: got %d, want %d", decoded.Speed, original.Speed)
	}
}

func TestFileTransferCompletePayloadRoundTrip(t *testing.T) {
	original := FileTransferCompletePayload{
		TransferID: "xfer-001",
		Checksum:   "sha256-abc123def456",
		Size:       1048576,
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var decoded FileTransferCompletePayload
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if decoded.TransferID != original.TransferID {
		t.Errorf("TransferID: got %q, want %q", decoded.TransferID, original.TransferID)
	}
	if decoded.Checksum != original.Checksum {
		t.Errorf("Checksum: got %q, want %q", decoded.Checksum, original.Checksum)
	}
	if decoded.Size != original.Size {
		t.Errorf("Size: got %d, want %d", decoded.Size, original.Size)
	}
}

func TestConnectPayloadPasswordOmit(t *testing.T) {
	p := ConnectPayload{HostID: "h-1"}
	data, err := json.Marshal(p)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var raw map[string]interface{}
	json.Unmarshal(data, &raw)
	if _, ok := raw["password"]; ok {
		t.Error("password should be omitted when empty")
	}
}

func TestMouseClickPayloadValues(t *testing.T) {
	cases := []MouseClickPayload{
		{Button: 0, X: 100.5, Y: 200.0, Down: true},   // left down
		{Button: 1, X: 0, Y: 0, Down: false},            // right up
		{Button: 2, X: -50, Y: -50, Down: true},         // middle down
	}
	for i, tc := range cases {
		data, err := json.Marshal(tc)
		if err != nil {
			t.Fatalf("case %d marshal: %v", i, err)
		}
		var decoded MouseClickPayload
		json.Unmarshal(data, &decoded)
		if decoded.Button != tc.Button || decoded.X != tc.X || decoded.Y != tc.Y || decoded.Down != tc.Down {
			t.Errorf("case %d: got Button=%d X=%f Y=%f Down=%v", i, decoded.Button, decoded.X, decoded.Y, decoded.Down)
		}
	}
}

func TestFileTransferErrorPayloadRoundTrip(t *testing.T) {
	original := FileTransferErrorPayload{
		TransferID: "xfer-001",
		Code:       "disk_full",
		Message:    "No space left on device",
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var decoded FileTransferErrorPayload
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if decoded.TransferID != original.TransferID {
		t.Errorf("TransferID: got %q, want %q", decoded.TransferID, original.TransferID)
	}
	if decoded.Code != original.Code {
		t.Errorf("Code: got %q, want %q", decoded.Code, original.Code)
	}
	if decoded.Message != original.Message {
		t.Errorf("Message: got %q, want %q", decoded.Message, original.Message)
	}
}
