// Package s3 provides S3-compatible storage providers for Marmot
// Supports AWS S3, Cloudflare R2, Backblaze B2, Wasabi, and any other S3-compatible service
package s3

import (
	"context"
	"fmt"
	"io"
	"time"
)

// BackupMetadata holds metadata about a backup
type BackupMetadata struct {
	BackupID        string    `json:"backup_id"`
	ServerID        string    `json:"server_id,omitempty"`
	DatabaseName    string    `json:"database_name,omitempty"`
	Size            int64     `json:"size"`
	CreatedAt       time.Time `json:"created_at"`
	RetentionTier   string    `json:"retention_tier,omitempty"`
	StorageLocation string    `json:"storage_location,omitempty"`
}

// Provider defines the interface for S3-compatible storage providers
type Provider interface {
	// Upload uploads a backup file to storage
	Upload(ctx context.Context, serverID, databaseName string, r io.Reader, timestamp time.Time) (*BackupMetadata, error)

	// Download downloads a backup file from storage
	Download(ctx context.Context, backupID string, w io.Writer) error

	// ListBackups lists all backups (optionally filtered by database)
	ListBackups(ctx context.Context, databaseName string, limit int) ([]BackupMetadata, error)

	// DeleteBackup deletes a backup from storage
	DeleteBackup(ctx context.Context, backupID string) error

	// HealthCheck checks storage connectivity
	HealthCheck(ctx context.Context) error

	// GetProviderType returns the type of storage provider (e.g., "s3", "r2", "b2")
	GetProviderType() string

	// GetEndpoint returns the configured endpoint URL
	GetEndpoint() string

	// GetBucket returns the configured bucket name
	GetBucket() string
}

// Config holds configuration for S3-compatible storage
type Config struct {
	Provider  string `mapstructure:"provider" yaml:"provider" json:"provider"`       // "s3", "r2", "b2", "wasabi", "minio", etc.
	Endpoint  string `mapstructure:"endpoint" yaml:"endpoint" json:"endpoint"`       // S3-compatible endpoint URL (optional for AWS S3)
	Bucket    string `mapstructure:"bucket" yaml:"bucket" json:"bucket"`             // Bucket name
	Region    string `mapstructure:"region" yaml:"region" json:"region"`             // Region (optional for some providers like R2)
	AccessKey string `mapstructure:"access_key" yaml:"access_key" json:"access_key"` // Access Key ID
	SecretKey string `mapstructure:"secret_key" yaml:"secret_key" json:"secret_key"` // Secret Access Key
	PathStyle bool   `mapstructure:"path_style" yaml:"path_style" json:"path_style"` // Use path-style addressing (needed for MinIO, some S3-compatible services)
	Prefix    string `mapstructure:"prefix" yaml:"prefix" json:"prefix"`             // Optional key prefix for all objects
	ServerID  string `mapstructure:"server_id" yaml:"server_id" json:"server_id"`    // Server identifier for metadata
}

// Validate checks if the S3 configuration is valid
func (c *Config) Validate() error {
	if c.Bucket == "" {
		return fmt.Errorf("bucket name is required")
	}

	if c.AccessKey == "" {
		return fmt.Errorf("access key is required")
	}

	if c.SecretKey == "" {
		return fmt.Errorf("secret key is required")
	}

	// Provider-specific defaults
	switch c.Provider {
	case "", "s3":
		// AWS S3 - endpoint and region are optional (will use defaults)
	case "r2":
		// Cloudflare R2 - endpoint is required, region is optional
		if c.Endpoint == "" {
			return fmt.Errorf("endpoint is required for R2")
		}
	case "b2":
		// Backblaze B2 - endpoint is required
		if c.Endpoint == "" {
			return fmt.Errorf("endpoint is required for B2")
		}
	case "wasabi":
		// Wasabi - endpoint is optional (will use default), region is required
		if c.Region == "" {
			c.Region = "us-east-1" // Wasabi default
		}
	case "minio":
		// MinIO - endpoint is required, path-style is typically needed
		if c.Endpoint == "" {
			return fmt.Errorf("endpoint is required for MinIO")
		}
		if !c.PathStyle {
			c.PathStyle = true // MinIO typically needs path-style
		}
	default:
		// Custom/other S3-compatible - endpoint is required
		if c.Endpoint == "" {
			return fmt.Errorf("endpoint is required for custom S3-compatible provider: %s", c.Provider)
		}
	}

	return nil
}

// ObjectMetadata holds metadata about a stored object
type ObjectMetadata struct {
	Key          string
	Size         int64
	LastModified time.Time
	ETag         string
	Metadata     map[string]string
}

// Common metadata keys used for backup objects
const (
	MetadataDatabaseName = "x-amz-meta-database-name"
	MetadataDatabaseType = "x-amz-meta-database-type"
	MetadataServerID     = "x-amz-meta-server-id"
	MetadataBackupID     = "x-amz-meta-backup-id"
	MetadataTimestamp    = "x-amz-meta-timestamp"
)

// GenerateObjectKey generates a standard object key for a backup
func GenerateObjectKey(prefix, serverID, databaseName string, timestamp time.Time) string {
	dateStr := timestamp.Format("2006/01/02")
	filename := fmt.Sprintf("%s-%d.enc", databaseName, timestamp.Unix())

	if prefix != "" {
		return fmt.Sprintf("%s/%s/%s/%s", prefix, serverID, dateStr, filename)
	}
	return fmt.Sprintf("%s/%s/%s", serverID, dateStr, filename)
}

// ProviderFactory is a function type for creating storage providers
type ProviderFactory func(ctx context.Context, config Config) (Provider, error)

var providerFactories = make(map[string]ProviderFactory)

// RegisterProvider registers a storage provider factory
func RegisterProvider(name string, factory ProviderFactory) {
	providerFactories[name] = factory
}

// GetProvider creates a storage provider based on configuration
func GetProvider(ctx context.Context, config Config) (Provider, error) {
	// Default to S3 if no provider specified
	if config.Provider == "" {
		config.Provider = "s3"
	}

	factory, ok := providerFactories[config.Provider]
	if !ok {
		// Try to use generic S3 provider for unknown types
		factory = providerFactories["s3"]
		if factory == nil {
			return nil, fmt.Errorf("unknown storage provider: %s", config.Provider)
		}
	}

	return factory(ctx, config)
}
