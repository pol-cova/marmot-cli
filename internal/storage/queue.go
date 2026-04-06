package storage

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
	
	_ "github.com/mattn/go-sqlite3"
)

// SQLiteQueue implements the Queue interface using SQLite
type SQLiteQueue struct {
	db *sql.DB
}

// canReadFile checks if a file is readable
func canReadFile(path string) bool {
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	f.Close()
	return true
}

// isRoot checks if the current user is root
func isRoot() bool {
	return os.Geteuid() == 0
}

// NewSQLiteQueue creates a new SQLite-based queue
func NewSQLiteQueue(dbPath string) (*SQLiteQueue, error) {
	// Check if database file exists
	_, err := os.Stat(dbPath)
	fileExists := err == nil
	
	// Check if we can write to the database file
	canWrite := false
	if fileExists {
		// Try to open file for writing to test permissions
		testFile, err := os.OpenFile(dbPath, os.O_RDWR, 0)
		if err == nil {
			testFile.Close()
			canWrite = true
		}
	}
	
	// Check directory permissions
	dir := filepath.Dir(dbPath)
	_, dirErr := os.Stat(dir)
	canWriteDir := false
	if dirErr == nil {
		// Check if directory is writable
		testFile := filepath.Join(dir, ".marmot-write-test")
		f, err := os.Create(testFile)
		if err == nil {
			f.Close()
			os.Remove(testFile)
			canWriteDir = true
		}
	}
	
	// Determine if we should try read-only mode first
	// Try read-only if:
	// 1. File exists but we can't write to it, OR
	// 2. Directory isn't writable, OR
	// 3. We're not root and file exists (safer to try read-only first)
	tryReadOnlyFirst := fileExists && (!canWrite || !canWriteDir || !isRoot())
	
	var db *sql.DB
	
	if tryReadOnlyFirst {
		// Check if file is readable before attempting to open
		if !canReadFile(dbPath) {
			return nil, fmt.Errorf("failed to open database: permission denied (hint: database file is owned by root. Run: sudo chmod 644 /var/lib/marmot/state.db to allow read access, or run 'marmot status' with sudo)")
		}
		
		// Try read-only mode first
		// The database might be in WAL mode which requires write access even for reads
		// Try multiple connection strategies
		connectionStrings := []string{
			dbPath + "?mode=ro&_query_only=1",
			dbPath + "?mode=ro&_journal_mode=DELETE",
			dbPath + "?mode=ro",
		}
		
		var lastErr error
		for _, connStr := range connectionStrings {
			db, err = sql.Open("sqlite3", connStr)
			if err != nil {
				lastErr = err
				continue
			}
			
			// Test if read-only connection works by trying a simple query
			var testVal int
			if err := db.QueryRow("SELECT 1").Scan(&testVal); err != nil {
				db.Close()
				lastErr = err
				continue
			}
			
			// Success - return the working connection
			return &SQLiteQueue{db: db}, nil
		}
		
		// All attempts failed - check if it's a permission error
		errStr := strings.ToLower(lastErr.Error())
		if strings.Contains(errStr, "permission denied") || strings.Contains(errStr, "unable to open database file") {
			return nil, fmt.Errorf("failed to open database: permission denied (hint: database file is owned by root. Run: sudo chmod 644 /var/lib/marmot/state.db to allow read access, or run 'marmot status' with sudo)")
		}
		
		// Otherwise, likely WAL mode issue
		return nil, fmt.Errorf("failed to open database in read-only mode: %w (hint: database is likely in WAL mode. Run: sudo sqlite3 /var/lib/marmot/state.db 'PRAGMA journal_mode=DELETE;' to convert it)", lastErr)
	}
	
	// Ensure directory exists (try even if file exists, in case we need to create it)
	if err := os.MkdirAll(dir, 0700); err != nil {
		// If directory creation fails and file doesn't exist, return error
		if !fileExists {
			return nil, fmt.Errorf("failed to create queue directory: %w", err)
		}
		// If file exists, try read-only mode
		if fileExists {
			db, err = sql.Open("sqlite3", dbPath+"?mode=ro&_query_only=1")
			if err != nil {
				db, err = sql.Open("sqlite3", dbPath+"?mode=ro")
				if err != nil {
					return nil, fmt.Errorf("failed to open database (read-only): %w", err)
				}
			}
			return &SQLiteQueue{db: db}, nil
		}
	}
	
	// Try to open with write access first
	// Use DELETE mode instead of WAL to allow read-only access later
	db, err = sql.Open("sqlite3", dbPath+"?_journal_mode=DELETE")
	if err != nil {
		// If opening fails and file exists, try read-only mode
		if fileExists {
			db, err = sql.Open("sqlite3", dbPath+"?mode=ro&_query_only=1")
			if err != nil {
				db, err = sql.Open("sqlite3", dbPath+"?mode=ro")
				if err != nil {
					return nil, fmt.Errorf("failed to open database: %w", err)
				}
			}
			return &SQLiteQueue{db: db}, nil
		}
		return nil, fmt.Errorf("failed to open database: %w", err)
	}
	
	// Explicitly set journal mode to DELETE to prevent WAL mode conversion
	// This must be done immediately after opening, before any other operations
	if _, err := db.Exec("PRAGMA journal_mode=DELETE"); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to set journal mode: %w", err)
	}
	
	queue := &SQLiteQueue{db: db}
	
	// Try to initialize schema - if this fails due to permissions, fall back to read-only
	if err := queue.init(); err != nil {
		// Check if error is due to read-only database
		errStr := strings.ToLower(err.Error())
		if fileExists && (strings.Contains(errStr, "readonly") || strings.Contains(errStr, "read-only") || strings.Contains(errStr, "permission denied") || strings.Contains(errStr, "unable to open") || strings.Contains(errStr, "attempt to write")) {
			// Close write connection and reopen in read-only mode
			db.Close()
			db, err = sql.Open("sqlite3", dbPath+"?mode=ro&_query_only=1")
			if err != nil {
				db, err = sql.Open("sqlite3", dbPath+"?mode=ro")
				if err != nil {
					return nil, fmt.Errorf("failed to open database (read-only): %w", err)
				}
			}
			queue.db = db
			// Read-only mode - skip initialization
		} else {
			db.Close()
			return nil, fmt.Errorf("failed to initialize queue: %w", err)
		}
	} else {
		// Successfully initialized - set file permissions (ignore errors if owned by another user)
		os.Chmod(dbPath, 0600)
	}
	
	return queue, nil
}

func (q *SQLiteQueue) init() error {
	query := `
	CREATE TABLE IF NOT EXISTS upload_queue (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT NOT NULL UNIQUE,
		path TEXT NOT NULL,
		size INTEGER NOT NULL,
		database_id TEXT NOT NULL,
		created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
		retry_count INTEGER NOT NULL DEFAULT 0,
		last_error TEXT,
		last_retry_at TIMESTAMP
	);

	CREATE INDEX IF NOT EXISTS idx_upload_queue_created_at ON upload_queue(created_at);
	`

	if _, err := q.db.Exec(query); err != nil {
		return err
	}

	// Migration: add last_retry_at for existing installations that don't have it yet
	// Intentionally ignore error — column may already exist
	_, _ = q.db.Exec(`ALTER TABLE upload_queue ADD COLUMN last_retry_at TIMESTAMP`)

	return nil
}

// Enqueue adds a backup to the upload queue
func (q *SQLiteQueue) Enqueue(backup BackupFile) error {
	query := `
	INSERT OR REPLACE INTO upload_queue (name, path, size, database_id, created_at)
	VALUES (?, ?, ?, ?, ?)
	`
	
	_, err := q.db.Exec(query, backup.Name, backup.Path, backup.Size, backup.DatabaseID, backup.CreatedAt)
	if err != nil {
		return fmt.Errorf("failed to enqueue backup: %w", err)
	}
	
	return nil
}

// Dequeue removes and returns the next backup from the queue
func (q *SQLiteQueue) Dequeue() (*BackupFile, error) {
	query := `
	SELECT name, path, size, database_id, created_at
	FROM upload_queue
	ORDER BY created_at ASC
	LIMIT 1
	`
	
	var backup BackupFile
	var createdAtStr string
	
	err := q.db.QueryRow(query).Scan(
		&backup.Name,
		&backup.Path,
		&backup.Size,
		&backup.DatabaseID,
		&createdAtStr,
	)
	
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to dequeue backup: %w", err)
	}
	
	backup.CreatedAt, _ = time.Parse(time.RFC3339, createdAtStr)
	
	// Remove from queue
	deleteQuery := `DELETE FROM upload_queue WHERE name = ?`
	if _, err := q.db.Exec(deleteQuery, backup.Name); err != nil {
		return nil, fmt.Errorf("failed to remove from queue: %w", err)
	}
	
	return &backup, nil
}

// Peek returns the next backup without removing it
func (q *SQLiteQueue) Peek() (*BackupFile, error) {
	query := `
	SELECT name, path, size, database_id, created_at
	FROM upload_queue
	ORDER BY created_at ASC
	LIMIT 1
	`
	
	var backup BackupFile
	var createdAtStr string
	
	err := q.db.QueryRow(query).Scan(
		&backup.Name,
		&backup.Path,
		&backup.Size,
		&backup.DatabaseID,
		&createdAtStr,
	)
	
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to peek queue: %w", err)
	}
	
	backup.CreatedAt, _ = time.Parse(time.RFC3339, createdAtStr)
	
	return &backup, nil
}

// List returns all queued backups
func (q *SQLiteQueue) List() ([]BackupFile, error) {
	query := `
	SELECT name, path, size, database_id, created_at, retry_count, last_error, last_retry_at
	FROM upload_queue
	ORDER BY created_at ASC
	`

	rows, err := q.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to list queue: %w", err)
	}
	defer rows.Close()

	var backups []BackupFile

	for rows.Next() {
		var backup BackupFile
		var createdAtStr string
		var lastError sql.NullString
		var lastRetryAtStr sql.NullString

		if err := rows.Scan(
			&backup.Name,
			&backup.Path,
			&backup.Size,
			&backup.DatabaseID,
			&createdAtStr,
			&backup.RetryCount,
			&lastError,
			&lastRetryAtStr,
		); err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}

		backup.CreatedAt, _ = time.Parse(time.RFC3339, createdAtStr)
		if lastRetryAtStr.Valid && lastRetryAtStr.String != "" {
			backup.LastRetryAt, _ = time.Parse(time.RFC3339, lastRetryAtStr.String)
		}
		backups = append(backups, backup)
	}

	return backups, nil
}

// IncrementRetry increments the retry count and updates last error for a queued item
func (q *SQLiteQueue) IncrementRetry(name string, err error) error {
	var errMsg string
	if err != nil {
		errMsg = err.Error()
	}

	query := `
	UPDATE upload_queue
	SET retry_count = retry_count + 1, last_error = ?, last_retry_at = ?
	WHERE name = ?
	`

	_, updateErr := q.db.Exec(query, errMsg, time.Now().UTC(), name)
	if updateErr != nil {
		return fmt.Errorf("failed to increment retry count: %w", updateErr)
	}

	return nil
}

// GetRetryCount returns the retry count for a queued item
func (q *SQLiteQueue) GetRetryCount(name string) (int, error) {
	query := `SELECT retry_count FROM upload_queue WHERE name = ?`
	
	var retryCount int
	err := q.db.QueryRow(query, name).Scan(&retryCount)
	if err != nil {
		return 0, fmt.Errorf("failed to get retry count: %w", err)
	}
	
	return retryCount, nil
}

// Remove removes a backup from the queue
func (q *SQLiteQueue) Remove(name string) error {
	query := `DELETE FROM upload_queue WHERE name = ?`
	
	result, err := q.db.Exec(query, name)
	if err != nil {
		return fmt.Errorf("failed to remove from queue: %w", err)
	}
	
	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return fmt.Errorf("backup not found in queue: %s", name)
	}
	
	return nil
}

// Clear removes all items from the queue
func (q *SQLiteQueue) Clear() error {
	query := `DELETE FROM upload_queue`
	
	_, err := q.db.Exec(query)
	if err != nil {
		return fmt.Errorf("failed to clear queue: %w", err)
	}
	
	return nil
}

// Close closes the database connection
func (q *SQLiteQueue) Close() error {
	return q.db.Close()
}

