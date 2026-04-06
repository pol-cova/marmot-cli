package agent

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"time"

	"github.com/pol-cova/marmot-cli/internal/backup"
	"github.com/pol-cova/marmot-cli/internal/config"
	"github.com/pol-cova/marmot-cli/internal/crypto"
	"github.com/pol-cova/marmot-cli/internal/remote"
	"github.com/pol-cova/marmot-cli/internal/storage"
)

// Agent orchestrates backup operations
type Agent struct {
	config     *config.Config
	storage    storage.Storage
	queue      storage.Queue
	remote     *remote.Storage
	encryptor  crypto.Encryptor
	compressor *backup.Compressor
}

// NewAgent creates a new agent instance
func NewAgent(cfg *config.Config, stor storage.Storage, q storage.Queue, remoteStorage *remote.Storage, enc crypto.Encryptor) *Agent {
	return &Agent{
		config:     cfg,
		storage:    stor,
		queue:      q,
		remote:     remoteStorage,
		encryptor:  enc,
		compressor: backup.NewCompressor(),
	}
}

// Backup performs a complete backup operation for a database
// If waitForUpload is true, it will wait for upload to complete before returning
func (a *Agent) Backup(ctx context.Context, dbConfig *config.DatabaseConfig, waitForUpload bool) error {
	// Create dumper based on database type
	var dumper backup.Dumper
	var err error

	switch dbConfig.Type {
	case "mysql":
		dumper, err = backup.NewMySQLDumper()
	case "postgres":
		dumper, err = backup.NewPostgreSQLDumper()
	case "mongo":
		dumper, err = backup.NewMongoDBDumper()
	default:
		return fmt.Errorf("unsupported database type: %s", dbConfig.Type)
	}

	if err != nil {
		return fmt.Errorf("failed to create dumper: %w", err)
	}

	// Create temporary files for pipeline
	dumpFile, err := os.CreateTemp("", "marmot-dump-*.sql")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	defer os.Remove(dumpFile.Name())
	defer dumpFile.Close()

	// Step 1: Dump database
	dumpConfig := backup.DumpConfig{
		Type:        dbConfig.Type,
		ContainerID: dbConfig.ContainerID,
		DSN:         dbConfig.DSN,
		Database:    dbConfig.Name,
		User:        dbConfig.User,
		Password:    dbConfig.Password,
		Host:        dbConfig.Host,
		Port:        dbConfig.Port,
	}

	if err := dumper.Dump(ctx, dumpFile, dumpConfig); err != nil {
		return fmt.Errorf("dump failed: %w", err)
	}

	if _, err := dumpFile.Seek(0, 0); err != nil {
		return fmt.Errorf("failed to seek dump file: %w", err)
	}

	// Step 2: Compress
	compressedFile, err := os.CreateTemp("", "marmot-compressed-*.gz")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	defer os.Remove(compressedFile.Name())
	defer compressedFile.Close()

	if err := a.compressor.Compress(dumpFile, compressedFile); err != nil {
		return fmt.Errorf("compression failed: %w", err)
	}

	if _, err := compressedFile.Seek(0, 0); err != nil {
		return fmt.Errorf("failed to seek compressed file: %w", err)
	}

	// Step 3: Encrypt
	encryptedFile, err := os.CreateTemp("", "marmot-encrypted-*.enc")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	defer os.Remove(encryptedFile.Name())
	defer encryptedFile.Close()

	if err := a.encryptor.Encrypt(compressedFile, encryptedFile); err != nil {
		return fmt.Errorf("encryption failed: %w", err)
	}

	if _, err := encryptedFile.Seek(0, 0); err != nil {
		return fmt.Errorf("failed to seek encrypted file: %w", err)
	}

	// Step 4: Save locally
	backupName := fmt.Sprintf("%s-%s-%d.enc", dbConfig.ID, dbConfig.Name, time.Now().Unix())
	backupPath, err := a.storage.Save(backupName, encryptedFile)
	if err != nil {
		return fmt.Errorf("storage failed: %w", err)
	}

	// Get file info for metadata
	fileInfo, err := os.Stat(backupPath)
	if err != nil {
		return fmt.Errorf("failed to stat backup file: %w", err)
	}

	// Step 5: Queue for upload (only for cloud storage)
	backupFile := storage.BackupFile{
		Name:       backupName,
		Path:       backupPath,
		Size:       fileInfo.Size(),
		CreatedAt:  time.Now(),
		DatabaseID: dbConfig.ID,
	}

	// For local-only storage, skip upload queue and complete immediately
	if a.config.IsLocal() {
		fmt.Printf("Backup completed: %s (local storage only)\n", backupName)
		return nil
	}

	// For cloud storage, queue and upload
	if err := a.queue.Enqueue(backupFile); err != nil {
		return fmt.Errorf("failed to enqueue backup: %w", err)
	}

	// Step 6: Upload to remote storage
	if waitForUpload {
		// Synchronous upload for manual backups
		if err := a.uploadBackupSync(ctx, backupFile, dbConfig); err != nil {
			return fmt.Errorf("upload failed: %w (backup saved locally)", err)
		}
	} else {
		// Asynchronous upload for scheduled backups
		go a.uploadBackup(ctx, backupFile, dbConfig)
	}

	return nil
}

// uploadBackupSync uploads a backup to remote storage synchronously (for manual backups)
func (a *Agent) uploadBackupSync(ctx context.Context, backupFile storage.BackupFile, dbConfig *config.DatabaseConfig) error {
	file, err := a.storage.Load(backupFile.Name)
	if err != nil {
		return fmt.Errorf("failed to load backup file: %w", err)
	}
	defer file.Close()

	// Upload using the remote storage client
	serverID := a.config.GetServerID()
	metadata, err := a.remote.GetClient().Upload(ctx, serverID, dbConfig.Name, file, backupFile.CreatedAt)
	if err != nil {
		return fmt.Errorf("upload to remote storage failed: %w", err)
	}

	// Remove from queue on success
	if err := a.queue.Remove(backupFile.Name); err != nil {
		return fmt.Errorf("failed to remove from queue: %w", err)
	}

	// Log success
	fmt.Printf("Uploaded to %s: backup_id=%s, size=%d bytes\n", a.remote.String(), metadata.BackupID, metadata.Size)

	return nil
}

// uploadBackup uploads a backup to remote storage asynchronously (for scheduled backups)
func (a *Agent) uploadBackup(ctx context.Context, backupFile storage.BackupFile, dbConfig *config.DatabaseConfig) {
	// Create a new context with timeout for async uploads
	uploadCtx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	file, err := a.storage.Load(backupFile.Name)
	if err != nil {
		fmt.Printf("Error: Failed to load backup file %s for upload: %v\n", backupFile.Name, err)
		// Increment retry count
		a.queue.IncrementRetry(backupFile.Name, err)
		return
	}
	defer file.Close()

	// Upload using the remote storage client
	serverID := a.config.GetServerID()
	metadata, err := a.remote.GetClient().Upload(uploadCtx, serverID, dbConfig.Name, file, backupFile.CreatedAt)
	if err != nil {
		fmt.Printf("Error: Upload failed for %s: %v (will retry later)\n", backupFile.Name, err)
		// Increment retry count
		a.queue.IncrementRetry(backupFile.Name, err)
		return
	}

	// Remove from queue on success
	if err := a.queue.Remove(backupFile.Name); err != nil {
		fmt.Printf("Warning: Failed to remove %s from queue: %v\n", backupFile.Name, err)
		return
	}

	fmt.Printf("Uploaded backup %s to %s: backup_id=%s\n", backupFile.Name, a.remote.String(), metadata.BackupID)
}

// RetryFailedUploads retries uploading backups that failed previously
func (a *Agent) RetryFailedUploads(ctx context.Context) error {
	const maxRetries = 5

	items, err := a.queue.List()
	if err != nil {
		return fmt.Errorf("failed to list queue: %w", err)
	}

	now := time.Now()

	for _, item := range items {
		if item.RetryCount >= maxRetries {
			fmt.Printf("Warning: Skipping %s - exceeded max retries (%d)\n", item.Name, maxRetries)
			continue
		}

		// Exponential backoff: 2^retryCount minutes (2, 4, 8, 16, 32 min)
		if item.RetryCount > 0 && !item.LastRetryAt.IsZero() {
			backoff := time.Duration(1<<uint(item.RetryCount)) * time.Minute
			if now.Before(item.LastRetryAt.Add(backoff)) {
				continue
			}
		}

		dbConfig := a.config.GetDatabaseByID(item.DatabaseID)
		if dbConfig == nil {
			fmt.Printf("Warning: Database config not found for %s\n", item.DatabaseID)
			continue
		}

		go a.uploadBackup(ctx, item, dbConfig)
	}

	return nil
}

// Restore restores a backup from remote storage to a database by backupID
func (a *Agent) Restore(ctx context.Context, dbConfig *config.DatabaseConfig, backupID string) error {
	// Download backup from remote storage
	downloadFile, err := os.CreateTemp("", "marmot-restore-*.enc")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	defer os.Remove(downloadFile.Name())
	defer downloadFile.Close()

	if err := a.remote.GetClient().Download(ctx, backupID, downloadFile); err != nil {
		return fmt.Errorf("download failed: %w", err)
	}

	if _, err := downloadFile.Seek(0, 0); err != nil {
		return fmt.Errorf("failed to seek download file: %w", err)
	}

	return a.restoreFromEncryptedReader(ctx, dbConfig, downloadFile)
}

// RestoreFromFile restores a backup from a local encrypted .enc file
func (a *Agent) RestoreFromFile(ctx context.Context, dbConfig *config.DatabaseConfig, encFilePath string) error {
	f, err := os.Open(encFilePath)
	if err != nil {
		return fmt.Errorf("failed to open encrypted file: %w", err)
	}
	defer f.Close()
	return a.restoreFromEncryptedReader(ctx, dbConfig, f)
}

// restoreFromEncryptedReader performs decrypt -> decompress -> restore pipeline from an encrypted reader
func (a *Agent) restoreFromEncryptedReader(ctx context.Context, dbConfig *config.DatabaseConfig, encrypted io.Reader) error {
	// Decrypt
	decryptedFile, err := os.CreateTemp("", "marmot-decrypted-*.gz")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	defer os.Remove(decryptedFile.Name())
	defer decryptedFile.Close()

	if err := a.encryptor.Decrypt(encrypted, decryptedFile); err != nil {
		return fmt.Errorf("decryption failed: %w", err)
	}

	if _, err := decryptedFile.Seek(0, 0); err != nil {
		return fmt.Errorf("failed to seek decrypted file: %w", err)
	}

	// Decompress
	decompressedFile, err := os.CreateTemp("", "marmot-decompressed-*.sql")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	defer os.Remove(decompressedFile.Name())
	defer decompressedFile.Close()

	if err := a.compressor.Decompress(decryptedFile, decompressedFile); err != nil {
		return fmt.Errorf("decompression failed: %w", err)
	}

	if _, err := decompressedFile.Seek(0, 0); err != nil {
		return fmt.Errorf("failed to seek decompressed file: %w", err)
	}

	// Restore to database
	return a.restoreToDatabase(ctx, dbConfig, decompressedFile)
}

func (a *Agent) restoreToDatabase(ctx context.Context, dbConfig *config.DatabaseConfig, r io.Reader) error {
	var cmd *exec.Cmd
	var stderrBuf bytes.Buffer

	switch dbConfig.Type {
	case "mysql":
		if dbConfig.ContainerID != "" {
			// Docker path
			args := []string{
				"exec", "-i", "-e", "MYSQL_PWD", dbConfig.ContainerID,
				"mysql",
				"-u", dbConfig.User,
				dbConfig.Name,
			}
			cmd = exec.CommandContext(ctx, "docker", args...)
			cmd.Env = append(os.Environ(), "MYSQL_PWD="+dbConfig.Password)
		} else {
			// Direct path
			args := []string{"-u", dbConfig.User}
			if dbConfig.Host != "" {
				args = append(args, "-h", dbConfig.Host)
			}
			if dbConfig.Port != 0 {
				args = append(args, "-P", strconv.Itoa(dbConfig.Port))
			}
			args = append(args, dbConfig.Name)
			cmd = exec.CommandContext(ctx, "mysql", args...)
			cmd.Env = append(os.Environ(), "MYSQL_PWD="+dbConfig.Password)
		}
		cmd.Stdin = r
		cmd.Stdout = io.Discard
		cmd.Stderr = &stderrBuf
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("mysql restore failed: %w\nstderr: %s", err, stderrBuf.String())
		}
		return nil

	case "postgres":
		if dbConfig.ContainerID != "" {
			// Docker path
			args := []string{
				"exec", "-i",
				"-e", "PGPASSWORD",
				dbConfig.ContainerID,
				"pg_restore",
				"-U", dbConfig.User,
			}
			if dbConfig.Host != "" {
				args = append(args, "-h", dbConfig.Host)
			}
			if dbConfig.Port != 0 {
				args = append(args, "-p", strconv.Itoa(dbConfig.Port))
			}
			args = append(args, "-d", dbConfig.Name, "-v")
			cmd = exec.CommandContext(ctx, "docker", args...)
			cmd.Env = append(os.Environ(), "PGPASSWORD="+dbConfig.Password)
		} else {
			// Direct path
			args := []string{"-U", dbConfig.User}
			if dbConfig.Host != "" {
				args = append(args, "-h", dbConfig.Host)
			}
			if dbConfig.Port != 0 {
				args = append(args, "-p", strconv.Itoa(dbConfig.Port))
			}
			args = append(args, "-d", dbConfig.Name, "-v")
			cmd = exec.CommandContext(ctx, "pg_restore", args...)
			cmd.Env = append(os.Environ(), "PGPASSWORD="+dbConfig.Password)
		}
		cmd.Stdin = r
		cmd.Stdout = io.Discard
		cmd.Stderr = &stderrBuf
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("postgres restore failed: %w\nstderr: %s", err, stderrBuf.String())
		}
		return nil

	case "mongo":
		authDB := dbConfig.Name
		if dbConfig.User == "root" {
			authDB = "admin"
		}
		if dbConfig.ContainerID != "" {
			// Docker path
			args := []string{
				"exec", "-i", dbConfig.ContainerID,
				"mongorestore",
				"--username", dbConfig.User,
				"--password", dbConfig.Password,
				"--authenticationDatabase", authDB,
				"--archive",
				"--drop",
			}
			if dbConfig.Host != "" {
				args = append(args, "--host", dbConfig.Host)
			}
			if dbConfig.Port != 0 {
				args = append(args, "--port", strconv.Itoa(dbConfig.Port))
			}
			cmd = exec.CommandContext(ctx, "docker", args...)
		} else {
			// Direct path
			args := []string{
				"--username", dbConfig.User,
				"--password", dbConfig.Password,
				"--authenticationDatabase", authDB,
				"--archive",
				"--drop",
			}
			if dbConfig.Host != "" {
				args = append(args, "--host", dbConfig.Host)
			}
			if dbConfig.Port != 0 {
				args = append(args, "--port", strconv.Itoa(dbConfig.Port))
			}
			cmd = exec.CommandContext(ctx, "mongorestore", args...)
		}
		cmd.Stdin = r
		cmd.Stdout = io.Discard
		cmd.Stderr = &stderrBuf
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("mongo restore failed: %w\nstderr: %s", err, stderrBuf.String())
		}
		return nil

	default:
		return fmt.Errorf("unsupported database type for restore: %s", dbConfig.Type)
	}
}

// GetRemoteStorage returns the remote storage instance
func (a *Agent) GetRemoteStorage() *remote.Storage {
	return a.remote
}
