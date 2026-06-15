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
