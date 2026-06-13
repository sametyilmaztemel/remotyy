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