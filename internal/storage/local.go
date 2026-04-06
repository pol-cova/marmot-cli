package storage

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// LocalStorage implements the Storage interface for local file storage
type LocalStorage struct {
	baseDir string
}

// NewLocalStorage creates a new local storage instance
func NewLocalStorage(baseDir string) (*LocalStorage, error) {
	if err := os.MkdirAll(baseDir, 0700); err != nil {
		return nil, fmt.Errorf("failed to create storage directory: %w", err)
	}
	
	return &LocalStorage{baseDir: baseDir}, nil
}

// Save saves a backup file locally
func (s *LocalStorage) Save(name string, r io.Reader) (string, error) {
	filePath := filepath.Join(s.baseDir, name)
	
	file, err := os.OpenFile(filePath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		return "", fmt.Errorf("failed to create file: %w", err)
	}
	defer file.Close()
	
	if _, err := io.Copy(file, r); err != nil {
		os.Remove(filePath)
		return "", fmt.Errorf("failed to write file: %w", err)
	}
	
	return filePath, nil
}

// Load loads a backup file
func (s *LocalStorage) Load(name string) (io.ReadCloser, error) {
	filePath := filepath.Join(s.baseDir, name)
	
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	
	return file, nil
}

// Delete deletes a backup file
func (s *LocalStorage) Delete(name string) error {
	filePath := filepath.Join(s.baseDir, name)
	
	if err := os.Remove(filePath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to delete file: %w", err)
	}
	
	return nil
}

// List lists all backup files
func (s *LocalStorage) List() ([]BackupFile, error) {
	entries, err := os.ReadDir(s.baseDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read directory: %w", err)
	}
	
	var files []BackupFile
	
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		
		info, err := entry.Info()
		if err != nil {
			continue
		}
		
		filePath := filepath.Join(s.baseDir, entry.Name())
		
		files = append(files, BackupFile{
			Name:      entry.Name(),
			Path:      filePath,
			Size:      info.Size(),
			CreatedAt: info.ModTime(),
		})
	}
	
	return files, nil
}

// GetDiskUsage returns disk usage information
func (s *LocalStorage) GetDiskUsage() (DiskUsage, error) {
	var usage DiskUsage
	
	// Get directory size
	var size int64
	err := filepath.Walk(s.baseDir, func(_ string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			size += info.Size()
		}
		return nil
	})
	
	if err != nil {
		return usage, fmt.Errorf("failed to calculate disk usage: %w", err)
	}
	
	usage.Used = size
	
	// Get available space (simplified - would need syscall for accurate values)
	stat, err := os.Stat(s.baseDir)
	if err != nil {
		return usage, fmt.Errorf("failed to stat directory: %w", err)
	}
	
	// This is a simplified implementation
	// In production, you'd use syscalls to get actual disk usage
	usage.Total = -1 // Unknown
	usage.Available = -1 // Unknown
	
	_ = stat // Suppress unused variable warning
	
	return usage, nil
}

