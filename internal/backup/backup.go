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
			rel, err := filepath.Rel(dataDir, path)
			if err != nil {
				return nil
			}
			if rel == BackupDirName || strings.HasPrefix(rel, BackupDirName+string(filepath.Separator)) {
				if rel != BackupDirName {
					return nil // skip backup files
				}
				return filepath.SkipDir
			}
			// Skip the backup file we're creating
			if path == backupPath {
				return nil
			}
			if info.IsDir() {
				return nil // tar handles directories via their contents
			}
			tarPath := "data/" + rel
			if err := addFileToTar(tw, path, tarPath); err != nil {
				return err
			}
			added = true
			return nil
		})
		if err != nil {
			return "", fmt.Errorf("walk data dir for backup: %w", err)
		}
	}

	if !added {
		os.Remove(backupPath)
		return "", fmt.Errorf("nothing to backup (no config or data files found)")
	}

	return backupPath, nil
}

// Restore restores a backup from the given backup file into the specified directory.
// backupPath is the path to the .tar.gz backup file.
// targetDir is the directory to restore into.
func Restore(backupPath string, targetDir string) error {
	f, err := os.Open(backupPath)
	if err != nil {
		return fmt.Errorf("open backup: %w", err)
	}
	defer f.Close()

	gr, err := gzip.NewReader(f)
	if err != nil {
		return fmt.Errorf("read gzip: %w", err)
	}
	defer gr.Close()

	tr := tar.NewReader(gr)

	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("read tar: %w", err)
		}

		// Determine target path
		var targetPath string
		switch {
		case header.Name == "config.yaml":
			targetPath = filepath.Join(targetDir, "remotty.yaml")
		case strings.HasPrefix(header.Name, "data/"):
			relPath := strings.TrimPrefix(header.Name, "data/")
			targetPath = filepath.Join(targetDir, relPath)
		default:
			targetPath = filepath.Join(targetDir, header.Name)
		}

		// Security: prevent path traversal
		if !strings.HasPrefix(filepath.Clean(targetPath), filepath.Clean(targetDir)) {
			return fmt.Errorf("path traversal detected: %s", header.Name)
		}

		if header.Typeflag == tar.TypeDir {
			if err := os.MkdirAll(targetPath, os.FileMode(header.Mode)); err != nil {
				return fmt.Errorf("create dir %s: %w", targetPath, err)
			}
			continue
		}

		// Create parent directories
		if err := os.MkdirAll(filepath.Dir(targetPath), 0700); err != nil {
			return fmt.Errorf("create parent dirs for %s: %w", targetPath, err)
		}

		outFile, err := os.OpenFile(targetPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, os.FileMode(header.Mode))
		if err != nil {
			return fmt.Errorf("create file %s: %w", targetPath, err)
		}

		if _, err := io.Copy(outFile, tr); err != nil {
			outFile.Close()
			return fmt.Errorf("write file %s: %w", targetPath, err)
		}
		outFile.Close()
	}

	return nil
}

// List returns all available backups for the given data directory.
func List(dataDir string) ([]Entry, error) {
	backupDir := filepath.Join(dataDir, BackupDirName)

	entries, err := os.ReadDir(backupDir)
	if err != nil {
		if os.IsNotExist(err) {
			return []Entry{}, nil
		}
		return nil, fmt.Errorf("read backup dir: %w", err)
	}

	var backups []Entry
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasPrefix(entry.Name(), "remotty_backup_") || !strings.HasSuffix(entry.Name(), ".tar.gz") {
			continue
		}

		info, err := entry.Info()
		if err != nil {
			continue
		}

		backups = append(backups, Entry{
			Path:    filepath.Join(backupDir, entry.Name()),
			Name:    entry.Name(),
			Size:    info.Size(),
			Created: info.ModTime(),
		})
	}

	// Sort by creation time, newest first
	sort.Slice(backups, func(i, j int) bool {
		return backups[i].Created.After(backups[j].Created)
	})

	return backups, nil
}

// Cleanup removes old backups, keeping at most `maxKeep` most recent ones
// and removing any older than `maxAge`.
func Cleanup(dataDir string, maxKeep int, maxAge time.Duration) (int, error) {
	backups, err := List(dataDir)
	if err != nil {
		return 0, err
	}

	if maxKeep <= 0 {
		maxKeep = MaxBackups
	}
	if maxAge <= 0 {
		maxAge = MaxBackupAge
	}

	cutoff := time.Now().Add(-maxAge)
	removed := 0

	for i, b := range backups {
		// Remove if too old or beyond the retention count
		if i >= maxKeep || b.Created.Before(cutoff) {
			if err := os.Remove(b.Path); err != nil {
				fmt.Fprintf(os.Stderr, "Warning: failed to remove old backup %s: %v\n", b.Path, err)
				continue
			}
			removed++
		}
	}

	return removed, nil
}

// addFileToTar adds a single file to the tar writer.
func addFileToTar(tw *tar.Writer, sourcePath string, tarPath string) error {
	info, err := os.Stat(sourcePath)
	if err != nil {
		return err
	}

	header, err := tar.FileInfoHeader(info, "")
	if err != nil {
		return err
	}
	header.Name = tarPath
	header.Format = tar.FormatPAX

	if err := tw.WriteHeader(header); err != nil {
		return err
	}

	f, err := os.Open(sourcePath)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = io.Copy(tw, f)
	return err
}
