package auth

import (
	"strings"
	"testing"
)

func TestHashAndCheckPassword(t *testing.T) {
	password := "super-secret-123"

	hash, err := HashPassword(password)
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}

	if hash == "" {
		t.Error("HashPassword returned empty hash")
	}

	// Hash should start with bcrypt prefix
	if !strings.HasPrefix(hash, "$2a$") {
		t.Errorf("Hash doesn't look like bcrypt: %s", hash[:10])
	}

	// Correct password should pass
	if !CheckPassword(password, hash) {
		t.Error("CheckPassword should return true for correct password")
	}

	// Wrong password should fail
	if CheckPassword("wrong-password", hash) {
		t.Error("CheckPassword should return false for wrong password")
	}
}

func TestHashPasswordArgon2(t *testing.T) {
	password := "argon2-test-password"

	hash, err := HashPasswordArgon2(password)
	if err != nil {
		t.Fatalf("HashPasswordArgon2: %v", err)
	}

	if !strings.HasPrefix(hash, "$argon2id$") {
		t.Errorf("Hash doesn't look like argon2id: %s", hash[:15])
	}
}

func TestHashPasswordDifferentHashes(t *testing.T) {
	password := "same-password"

	hash1, _ := HashPassword(password)
	hash2, _ := HashPassword(password)

	// Same password should produce different hashes (due to salt)
	if hash1 == hash2 {
		t.Error("Two hashes of the same password should differ (bcrypt salt)")
	}

	// But both should verify
	if !CheckPassword(password, hash1) {
		t.Error("hash1 should verify")
	}
	if !CheckPassword(password, hash2) {
		t.Error("hash2 should verify")
	}
}

func TestGenerateToken(t *testing.T) {
	token, err := GenerateToken(32)
	if err != nil {
		t.Fatalf("GenerateToken: %v", err)
	}

	// hex encoding doubles the length
	if len(token) != 64 {
		t.Errorf("token length = %d, want 64 (32 bytes hex)", len(token))
	}
}

func TestGenerateTokenUniqueness(t *testing.T) {
	t1, _ := GenerateToken(16)
	t2, _ := GenerateToken(16)

	if t1 == t2 {
		t.Error("Two generated tokens should not be equal")
	}
}

func TestValidateDeviceID(t *testing.T) {
	tests := []struct {
		name      string
		deviceID  string
		allowList []string
		want      bool
	}{
		{
			name:      "empty allow list allows all",
			deviceID:  "device-1",
			allowList: nil,
			want:      true,
		},
		{
			name:      "device in allow list",
			deviceID:  "device-1",
			allowList: []string{"device-1", "device-2"},
			want:      true,
		},
		{
			name:      "device not in allow list",
			deviceID:  "device-3",
			allowList: []string{"device-1", "device-2"},
			want:      false,
		},
		{
			name:      "wildcard allows all",
			deviceID:  "any-device",
			allowList: []string{"*"},
			want:      true,
		},
		{
			name:      "empty string device ID",
			deviceID:  "",
			allowList: []string{"device-1"},
			want:      false,
		},
		{
			name:      "empty string device ID with empty list",
			deviceID:  "",
			allowList: nil,
			want:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ValidateDeviceID(tt.deviceID, tt.allowList)
			if got != tt.want {
				t.Errorf("ValidateDeviceID(%q, %v) = %v, want %v",
					tt.deviceID, tt.allowList, got, tt.want)
			}
		})
	}
}

func TestCheckPasswordEmptyHash(t *testing.T) {
	// Empty hash should not panic
	result := CheckPassword("test", "")
	if result {
		t.Error("CheckPassword with empty hash should return false")
	}
}
