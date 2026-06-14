package protocol

// Error codes for structured error responses.
// Ranges:
//   1000-1999: Authentication & Authorization
//   2000-2999: Connection & Transport
//   3000-3999: Protocol & Message
//   4000-4999: Session & Resource
//   5000-5999: Internal Server
const (
	// Auth errors (1xxx)
	ErrUnauthorized      = 1001 // Missing or invalid auth token
	ErrAuthFailed        = 1002 // Wrong master password
	ErrAuthRequired      = 1003 // Auth required but not provided

	// Connection errors (2xxx)
	ErrHostNotFound      = 2001 // Requested host is offline/unknown
	ErrRoomNotFound      = 2002 // Room does not exist
	ErrPeerDisconnected  = 2003 // Peer has disconnected
	ErrRateLimited       = 2004 // Too many requests

	// Protocol errors (3xxx)
	ErrInvalidPayload    = 3001 // Malformed or unparseable payload
	ErrInvalidMessage    = 3002 // Malformed JSON message
	ErrUnknownType       = 3003 // Unknown message type
	ErrPayloadTooLarge   = 3004 // Message exceeds size limit

	// Session errors (4xxx)
	ErrInvalidState      = 4001 // Operation not valid in current state
	ErrMaxSessions       = 4002 // Max concurrent sessions reached
	ErrNotAllowed        = 4003 // Client not in allow list
	ErrFeatureDisabled   = 4004 // Requested feature not enabled

	// Internal errors (5xxx)
	ErrInternal          = 5001 // Unexpected server error
	ErrShutdown          = 5002 // Server is shutting down
)

// ErrorPayload represents a structured error message.
type ErrorPayload struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}
