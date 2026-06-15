// Package protocol defines all wire-format messages for remotty signaling and data channels.
package protocol

import "encoding/json"

// ======== Message Types ========

type MessageType string

const (
	// Signaling: Host → Server
	MsgRegister  MessageType = "register"
	MsgHeartbeat MessageType = "heartbeat"
	MsgUpdate    MessageType = "update" // host updates its capabilities

	// Signaling: Client → Server
	MsgListHosts MessageType = "list_hosts"
	MsgConnect   MessageType = "connect"

	// Signaling: Server → Peers
	MsgHostList  MessageType = "host_list"
	MsgRoomReady MessageType = "room_ready"
	MsgPeerLeft  MessageType = "peer_left"

	// WebRTC Negotiation (relayed through server)
	MsgOffer      MessageType = "offer"
	MsgAnswer     MessageType = "answer"
	MsgICECandidate MessageType = "ice_candidate"
	MsgRenegotiate MessageType = "renegotiate"

	// Data Channel — Auth
	MsgAuth     MessageType = "auth"
	MsgAuthOK   MessageType = "auth_ok"
	MsgAuthFail MessageType = "auth_fail"

	// Data Channel — Terminal
	MsgInput  MessageType = "input"
	MsgOutput MessageType = "output"
	MsgResize MessageType = "resize"

	// Data Channel — Screen
	MsgScreenStart  MessageType = "screen_start"
	MsgScreenStop   MessageType = "screen_stop"
	MsgScreenFrame  MessageType = "screen_frame"
	MsgScreenResize MessageType = "screen_resize"

	// Data Channel — Input Events
	MsgMouseMove   MessageType = "mouse_move"
	MsgMouseClick  MessageType = "mouse_click"
	MsgMouseScroll MessageType = "mouse_scroll"
	MsgKeyPress    MessageType = "key_press"
	MsgKeyRelease  MessageType = "key_release"

	// Data Channel — File Transfer
	MsgFileRequest  MessageType = "file_request"
	MsgFileAccept   MessageType = "file_accept"
	MsgFileReject   MessageType = "file_reject"
	MsgFileChunk    MessageType = "file_chunk"
	MsgFileComplete MessageType = "file_complete"
	MsgFileError     MessageType = "file_error"
	MsgFileProgress MessageType = "file_progress"
	MsgFileCancel   MessageType = "file_cancel"

	// Data Channel — Clipboard
	MsgClipboard        MessageType = "clipboard"
	MsgClipboardData    MessageType = "clipboard_data"
	MsgClipboardRequest MessageType = "clipboard_request"

	// Data Channel — Keepalive
	MsgPing MessageType = "ping"
	MsgPong MessageType = "pong"

	// Error
	MsgError MessageType = "error"
)

// ======== Envelope ========

// Message is the envelope for all WebSocket and DataChannel messages.
type Message struct {
	Type    MessageType     `json:"type"`
	Payload json.RawMessage `json:"payload,omitempty"`
	From    string          `json:"from,omitempty"`
	To      string          `json:"to,omitempty"`
	Room    string          `json:"room,omitempty"`
	ID      string          `json:"id,omitempty"` // message ID for dedup
	Time    int64           `json:"time,omitempty"` // unix timestamp
}

// NewMessage creates a message with the given type and payload.
func NewMessage(t MessageType, payload interface{}) Message {
	data, _ := json.Marshal(payload)
	return Message{
		Type:    t,
		Payload: data,
	}
}

// ======== Payloads ========

// RegisterPayload is sent by host on connect.
type RegisterPayload struct {
	Name      string   `json:"name"`
	Platform  string   `json:"platform"`
	Arch      string   `json:"arch"`
	Version   string   `json:"version"`
	Features  []string `json:"features"`
	DeviceID  string   `json:"device_id,omitempty"`
}

// HostInfo describes a registered host.
type HostInfo struct {
	ID        string   `json:"id"`
	Name      string   `json:"name"`
	Platform  string   `json:"platform"`
	Arch      string   `json:"arch"`
	Version   string   `json:"version"`
	Online    bool     `json:"online"`
	Features  []string `json:"features"`
	DeviceID  string   `json:"device_id,omitempty"`
	Ping      int      `json:"ping,omitempty"` // latency in ms
}

// ConnectPayload is sent by client to request a host connection.
type ConnectPayload struct {
	HostID   string `json:"host_id"`
	Password string `json:"password,omitempty"`
}

// AuthPayload authenticates to a host session.
type AuthPayload struct {
	Password string `json:"password"`
}

// ResizePayload for terminal window changes.
type ResizePayload struct {
	Rows uint16 `json:"rows"`
	Cols uint16 `json:"cols"`
}

// FileRequestPayload initiates a file transfer.
type FileRequestPayload struct {
	TransferID string `json:"transfer_id"`
	Name       string `json:"name"`
	Size       int64  `json:"size"`
	MimeType   string `json:"mime_type"`
	ChunkSize  int    `json:"chunk_size"`
}

// FileChunkPayload is a chunk of file data.
type FileChunkPayload struct {
	TransferID string `json:"transfer_id"`
	Index      int    `json:"index"`