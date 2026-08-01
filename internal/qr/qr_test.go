package qr

import (
	"testing"
)

func TestPairingURLEncode(t *testing.T) {
	p := PairingURL{
		Version:  1,
		Signal:   "ws://192.168.1.100:9000",
		HostID:   "p-123",
		HostName: "test-host",
		Token:    "secret",
	}

	url := p.Encode()
	if url == "" {
		t.Error("encoded URL should not be empty")
	}
	if len(url) < 20 {
		t.Errorf("URL seems too short: %s", url)
	}
}

func TestPairingURLRoundtrip(t *testing.T) {
	original := PairingURL{
		Version:  1,
		Signal:   "ws://192.168.1.100:9000",
		HostID:   "p-456",
		HostName: "roundtrip-host",
		Token:    "",
	}

	url := original.Encode()
	decoded, err := DecodeURL(url)
	if err != nil {
		t.Fatalf("DecodeURL: %v", err)
	}

	if decoded.Version != original.Version {
		t.Errorf("version = %d, want %d", decoded.Version, original.Version)
	}
	if decoded.Signal != original.Signal {
		t.Errorf("signal = %q, want %q", decoded.Signal, original.Signal)
	}
	if decoded.HostID != original.HostID {
		t.Errorf("hostID = %q, want %q", decoded.HostID, original.HostID)
	}
	if decoded.HostName != original.HostName {
		t.Errorf("hostName = %q, want %q", decoded.HostName, original.HostName)
	}
}

func TestDecodeURLInvalid(t *testing.T) {
	_, err := DecodeURL("https://example.com")
	if err == nil {
		t.Error("invalid URL should return error")
	}

	_, err = DecodeURL("remotty://connect/invalid-json")
	if err == nil {
		t.Error("invalid JSON payload should return error")
	}
}

func TestGenerate(t *testing.T) {
	p := PairingURL{
		Version:  1,
		Signal:   "ws://localhost:9000",
		HostID:   "p-qr-test",
		HostName: "qr-host",
	}

	qrArt, url, err := Generate(p)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if qrArt == "" {
		t.Error("QR art should not be empty")
	}
	if url == "" {
		t.Error("URL should not be empty")
	}
}

func TestGenerateSmall(t *testing.T) {
	p := PairingURL{
		Version:  1,
		Signal:   "ws://localhost:9000",
		HostID:   "p-small-qr",
		HostName: "small-qr-host",
	}

	qrArt, url, err := GenerateSmall(p)
	if err != nil {
		t.Fatalf("GenerateSmall: %v", err)
	}
	if qrArt == "" {
		t.Error("QR art should not be empty")
	}
	if url == "" {
		t.Error("URL should not be empty")
	}
}

func TestPairingURLWithToken(t *testing.T) {
	p := PairingURL{
		Version:  1,
		Signal:   "wss://remotty.example.com",
		HostID:   "p-token",
		HostName: "token-host",
		Token:    "my-secret-token",
	}

	url := p.Encode()
	decoded, err := DecodeURL(url)
	if err != nil {
		t.Fatalf("DecodeURL: %v", err)
	}
	if decoded.Token != "my-secret-token" {
		t.Errorf("token = %q, want %q", decoded.Token, "my-secret-token")
	}
}
