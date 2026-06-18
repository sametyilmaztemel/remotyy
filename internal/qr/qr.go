// Package qr provides QR code generation for zero-config host pairing.
package qr

import (
	"encoding/json"
	"fmt"
	"strings"

	qrcode "github.com/skip2/go-qrcode"
)

// PairingURL contains all info needed for a client to connect.
type PairingURL struct {
	Version  int    `json:"v"`
	Signal   string `json:"signal"`
	HostID   string `json:"host"`
	HostName string `json:"name"`
	Token    string `json:"token,omitempty"`
}

// Encode creates a remotty:// URL from pairing info.