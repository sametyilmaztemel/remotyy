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
func (p PairingURL) Encode() string {
	data, _ := json.Marshal(p)
	return fmt.Sprintf("remotty://connect/%s", strings.TrimRight(string(data), "\n"))
}

// Generate creates a QR code as an ANSI terminal string.
// Returns the QR code art and the raw URL.
func Generate(p PairingURL) (qrArt, url string, err error) {
	url = p.Encode()

	// Generate QR as terminal output (ANSI blocks)
	code, err := qrcode.New(url, qrcode.Medium)
	if err != nil {
		return "", "", fmt.Errorf("generate qr: %w", err)
	}

	qrArt = code.ToString(true)
	return qrArt, url, nil
}

// GenerateSmall creates a smaller QR (2x smaller) for terminals.
func GenerateSmall(p PairingURL) (qrArt, url string, err error) {