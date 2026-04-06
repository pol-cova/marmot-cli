package storage

import (
	"io"
	"time"
)

// Storage defines the interface for backup storage
type Storage interface {
	// Save saves a backup file locally
	Save(name string, r io.Reader) (string, error)
	
	// Load loads a backup file
	Load(name string) (io.ReadCloser, error)
	
	// Delete deletes a backup file
	Delete(name string) error
	
	// List lists all backup files
	List() ([]BackupFile, error)
	
	// GetDiskUsage returns disk usage information
	GetDiskUsage() (DiskUsage, error)
}

// BackupFile represents a backup file
type BackupFile struct {
	Name        string
	Path        string
	Size        int64
	CreatedAt   time.Time
	DatabaseID  string
	RetryCount  int
	LastRetryAt time.Time
}

// DiskUsage represents disk usage information
type DiskUsage struct {
	Total     int64
	Used      int64
	Available int64
}

// Queue defines the interface for upload queue management
type Queue interface {
	// Enqueue adds a backup to the upload queue
	Enqueue(backup BackupFile) error
	
	// Dequeue removes and returns the next backup from the queue
	Dequeue() (*BackupFile, error)
	
	// Peek returns the next backup without removing it
	Peek() (*BackupFile, error)
	
	// List returns all queued backups
	List() ([]BackupFile, error)
	
	// Remove removes a backup from the queue
	Remove(name string) error
	
	// Clear removes all items from the queue
	Clear() error
	
	// IncrementRetry increments the retry count and updates last error for a queued item
	IncrementRetry(name string, err error) error
	
	// GetRetryCount returns the retry count for a queued item
	GetRetryCount(name string) (int, error)
}

