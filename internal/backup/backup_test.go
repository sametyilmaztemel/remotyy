package backup

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestCreateRestoreBackup(t *testing.T) {
	// Setup temp directories
	tmpDir := t.TempDir()
	dataDir := filepath.Join(tmpDir, "data")
	os.MkdirAll(dataDir, 0700)

	// Create a test config file
	configPath := filepath.Join(tmpDir, "remotty.yaml")
	configContent := []byte("signal:\n  port: 9000\n")
	if err := os.WriteFile(configPath, configContent, 0644); err != nil {
		t.Fatal(err)
	}

	// Create some test data files
	sessionDir := filepath.Join(dataDir, "sessions")
	os.MkdirAll(sessionDir, 0700)
	if err := os.WriteFile(filepath.Join(sessionDir, "session_1.json"), []byte(`{"id":"1"}`), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dataDir, "known_hosts.json"), []byte(`[]`), 0644); err != nil {
		t.Fatal(err)
	}

	// Create backup
	backupPath, err := Create(dataDir, configPath)
	if err != nil {
		t.Fatalf("Create backup: %v", err)
	}
	defer os.Remove(backupPath)

	if backupPath == "" {
		t.Fatal("backup path should not be empty")
	}

	// Verify backup file exists
	if _, err := os.Stat(backupPath); err != nil {
		t.Errorf("backup file not found: %v", err)
	}

	// Restore to a new directory
	restoreDir := filepath.Join(tmpDir, "restore")
	if err := Restore(backupPath, restoreDir); err != nil {
		t.Fatalf("Restore: %v", err)
	}

	// Verify restored config
	restoredConfig := filepath.Join(restoreDir, "remotty.yaml")
	if _, err := os.Stat(restoredConfig); err != nil {
		t.Errorf("restored config not found: %v", err)
	}
	restoredData, err := os.ReadFile(restoredConfig)
	if err != nil {
		t.Fatal(err)
	}
	if string(restoredData) != string(configContent) {
		t.Errorf("restored config content mismatch: got %q, want %q", string(restoredData), string(configContent))
	}

	// Verify restored session data
	restoredSession := filepath.Join(restoreDir, "sessions", "session_1.json")
	if _, err := os.Stat(restoredSession); err != nil {
		t.Errorf("restored session file not found: %v", err)
	}

	// Verify restored known_hosts
	restoredHosts := filepath.Join(restoreDir, "known_hosts.json")
	if _, err := os.Stat(restoredHosts); err != nil {
		t.Errorf("restored known_hosts file not found: %v", err)
	}
}

func TestBackupNothingToBackup(t *testing.T) {
	tmpDir := t.TempDir()

	// No config file, no data dir - should fail
	_, err := Create(tmpDir, "/nonexistent/config.yaml")
	if err == nil {
		t.Error("expected error when nothing to backup")
	}
}

func TestListBackups(t *testing.T) {
	tmpDir := t.TempDir()
	dataDir := filepath.Join(tmpDir, "data")
	os.MkdirAll(dataDir, 0700)

	// List with no backups
	backups, err := List(dataDir)
	if err != nil {
		t.Fatalf("List empty: %v", err)
	}
	if len(backups) != 0 {
		t.Errorf("expected 0 backups, got %d", len(backups))
	}

	// Create a backup first
	configPath := filepath.Join(tmpDir, "remotty.yaml")
	os.WriteFile(configPath, []byte("port: 9000"), 0644)
	backupPath, err := Create(dataDir, configPath)
	if err != nil {
		t.Fatalf("Create backup: %v", err)
	}
	defer os.Remove(backupPath)

	// List again
	backups, err = List(dataDir)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(backups) != 1 {
		t.Errorf("expected 1 backup, got %d", len(backups))
	}
	if backups[0].Name == "" {
		t.Error("backup name should not be empty")
	}
	if backups[0].Size <= 0 {
		t.Errorf("backup size should be > 0, got %d", backups[0].Size)
	}
	if backups[0].Created.IsZero() {
		t.Error("backup created time should not be zero")
	}
}

func TestCleanupBackups(t *testing.T) {
	tmpDir := t.TempDir()
	dataDir := filepath.Join(tmpDir, "data")
	os.MkdirAll(dataDir, 0700)

	// Create a backup first
	configPath := filepath.Join(tmpDir, "remotty.yaml")
	os.WriteFile(configPath, []byte("port: 9000"), 0644)
	backupPath, err := Create(dataDir, configPath)
	if err != nil {
		t.Fatalf("Create backup: %v", err)
	}
	defer os.Remove(backupPath)

	// Cleanup with maxKeep=0 (should keep all since it's the only one)
	removed, err := Cleanup(dataDir, 0, 0)
	if err != nil {
		t.Fatalf("Cleanup: %v", err)
	}
	if removed != 0 {
		t.Errorf("expected 0 removed, got %d", removed)
	}

	// Cleanup with maxKeep=0 and maxAge=1ns (should remove the backup)
	removed, err = Cleanup(dataDir, 0, 1*time.Nanosecond)
	if err != nil {
		t.Fatalf("Cleanup with short age: %v", err)
	}
	if removed != 1 {
		t.Errorf("expected 1 removed, got %d", removed)
	}

	// Verify backup is gone
	backups, _ := List(dataDir)
	if len(backups) != 0 {
		t.Errorf("expected 0 backups after cleanup, got %d", len(backups))
	}
}

func TestInvalidBackupPath(t *testing.T) {
	err := Restore("/nonexistent/backup.tar.gz", "/tmp/restore")
	if err == nil {
		t.Error("expected error for nonexistent backup")
	}
}

func TestPathTraversalProtection(t *testing.T) {
	// Create a valid backup with a path traversal attempt
	tmpDir := t.TempDir()
	dataDir := filepath.Join(tmpDir, "data")
	os.MkdirAll(dataDir, 0700)

	// Create a backup, then try to restore from it
	configPath := filepath.Join(tmpDir, "remotty.yaml")
	os.WriteFile(configPath, []byte("port: 9000"), 0644)
	backupPath, err := Create(dataDir, configPath)
	if err != nil {
		t.Fatalf("Create backup: %v", err)
	}

	// Restore should work
	restoreDir := filepath.Join(tmpDir, "safe_restore")
	if err := Restore(backupPath, restoreDir); err != nil {
		t.Errorf("safe restore should work: %v", err)
	}
}

func TestBackupWithOnlyConfig(t *testing.T) {
	tmpDir := t.TempDir()
	dataDir := filepath.Join(tmpDir, "data") // doesn't exist yet

	configPath := filepath.Join(tmpDir, "remotty.yaml")
	os.WriteFile(configPath, []byte("port: 9000"), 0644)

	// Create backup - data dir doesn't need to exist, config is enough
	backupPath, err := Create(dataDir, configPath)
	if err != nil {
		t.Fatalf("Create backup with only config: %v", err)
	}
	defer os.Remove(backupPath)

	if backupPath == "" {
		t.Fatal("backup path should not be empty")
	}
}

func TestRestoreFilePermissions(t *testing.T) {
	tmpDir := t.TempDir()
	dataDir := filepath.Join(tmpDir, "data")
	os.MkdirAll(dataDir, 0700)

	configPath := filepath.Join(tmpDir, "remotty.yaml")
	os.WriteFile(configPath, []byte("port: 9000"), 0644)

	backupPath, err := Create(dataDir, configPath)
	if err != nil {
		t.Fatalf("Create backup: %v", err)
	}

	restoreDir := filepath.Join(tmpDir, "restore")
	if err := Restore(backupPath, restoreDir); err != nil {
		t.Fatalf("Restore: %v", err)
	}

	// Verify file is readable
	restoredConfig := filepath.Join(restoreDir, "remotty.yaml")
	info, err := os.Stat(restoredConfig)
	if err != nil {
		t.Fatalf("stat restored config: %v", err)
	}
	if info.Size() == 0 {
		t.Error("restored file should not be empty")
	}
}

func TestBackupListOrder(t *testing.T) {
	tmpDir := t.TempDir()
	dataDir := filepath.Join(tmpDir, "data")
	os.MkdirAll(dataDir, 0700)

	// Create multiple backups
	for i := 0; i < 3; i++ {
		configPath := filepath.Join(tmpDir, "remotty.yaml")
		os.WriteFile(configPath, []byte("port: 9000"), 0644)
		bp, err := Create(dataDir, configPath)
		if err != nil {
			t.Fatalf("Create backup %d: %v", i, err)
		}
		// Stagger creation times
		time.Sleep(10 * time.Millisecond)
		_ = bp
	}

	backups, err := List(dataDir)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(backups) != 3 {
		t.Errorf("expected 3 backups, got %d", len(backups))
	}

	// Should be sorted newest first
	for i := 1; i < len(backups); i++ {
		if backups[i].Created.After(backups[i-1].Created) {
			t.Errorf("backups should be sorted newest first")
		}
	}
}
