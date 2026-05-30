// Package backup provides backup and restore functionality for remotty
// configuration and session data.
package backup

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const (
	// BackupDirName is the name of the backups subdirectory within data_dir.
	BackupDirName = "backups"
	// MaxBackups is the default maximum number of backups to retain.
	MaxBackups = 10
	// MaxBackupAge is the default maximum age for backups (30 days).
	MaxBackupAge = 30 * 24 * time.Hour
)

// Entry describes a single backup.
type Entry struct {
	Path    string    `json:"path"`
	Name    string    `json:"name"`
	Size    int64     `json:"size"`
	Created time.Time `json:"created"`
}

// Create creates a backup of the data directory and config file.
// dataDir is the remotty data directory (e.g., ~/.remotty).
// configPath is the path to the config file (optional).
// Returns the path to the created backup file.
func Create(dataDir string, configPath string) (string, error) {
	backupDir := filepath.Join(dataDir, BackupDirName)
	if err := os.MkdirAll(backupDir, 0700); err != nil {
		return "", fmt.Errorf("create backup dir: %w", err)
	}

	timestamp := time.Now().UTC().Format("20060102_150405.000000000")
	backupName := fmt.Sprintf("remotty_backup_%s.tar.gz", timestamp)
	backupPath := filepath.Join(backupDir, backupName)

	f, err := os.Create(backupPath)
	if err != nil {
		return "", fmt.Errorf("create backup file: %w", err)
	}
	defer f.Close()

	gw := gzip.NewWriter(f)
	defer gw.Close()

	tw := tar.NewWriter(gw)
	defer tw.Close()

	added := false

	// Add config file if it exists
	if configPath != "" {
		if info, err := os.Stat(configPath); err == nil && !info.IsDir() {
			if err := addFileToTar(tw, configPath, "config.yaml"); err != nil {
				return "", fmt.Errorf("add config to backup: %w", err)
			}
			added = true
		}
	}

	// Try common config paths
	commonPaths := []string{
		filepath.Join(dataDir, "remotty.yaml"),
		filepath.Join(dataDir, "config.yaml"),
		filepath.Join(dataDir, "remotty.yml"),
	}
	for _, p := range commonPaths {
		if p == configPath {
			continue // already added above
		}
		if info, err := os.Stat(p); err == nil && !info.IsDir() {
			if err := addFileToTar(tw, p, filepath.Base(p)); err != nil {
				return "", fmt.Errorf("add config to backup: %w", err)
			}
			added = true
			break
		}
	}

	// Add data directory contents (excluding backups dir and backup files)
	dataDirInfo, err := os.Stat(dataDir)
	if err != nil {
		if !added {
			return "", fmt.Errorf("data dir not found: %w", err)
		}
		// If we at least have the config file, that's fine
	} else if dataDirInfo.IsDir() {
		err = filepath.Walk(dataDir, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return nil // skip inaccessible files
			}
			// Skip the backup directory itself