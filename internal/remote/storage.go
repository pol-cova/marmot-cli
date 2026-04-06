// Package remote provides S3-compatible storage for Marmot
// Supports AWS S3, Cloudflare R2, Backblaze B2, Wasabi, MinIO, and other S3-compatible services
package remote

import (
	"context"
	"fmt"

	"github.com/pol-cova/marmot-cli/internal/config"
	"github.com/pol-cova/marmot-cli/internal/s3"
)

// Storage provides S3-compatible storage operations
type Storage struct {
	provider s3.Provider
	config   s3.Config
}

// NewStorage creates a new S3-compatible storage instance
// Returns error if config is for local storage (not S3)
func NewStorage(cfg *config.Config) (*Storage, error) {
	return NewStorageWithContext(context.Background(), cfg)
}

// NewStorageWithContext creates a new S3-compatible storage instance with context
// Returns error if config is for local storage (not S3)
func NewStorageWithContext(ctx context.Context, cfg *config.Config) (*Storage, error) {
	if !cfg.IsS3() {
		return nil, fmt.Errorf("remote storage not configured: using local storage mode")
	}

	if err := cfg.S3.Validate(); err != nil {
		return nil, fmt.Errorf("invalid S3 configuration: %w", err)
	}

	provider, err := s3.GetProvider(ctx, cfg.S3)
	if err != nil {
		return nil, fmt.Errorf("failed to create storage provider: %w", err)
	}

	return &Storage{
		provider: provider,
		config:   cfg.S3,
	}, nil
}

// GetClient returns the underlying S3 provider client
func (s *Storage) GetClient() s3.Provider {
	return s.provider
}

// GetProviderType returns the type of storage provider (e.g., "s3", "r2", "b2")
func (s *Storage) GetProviderType() string {
	return s.provider.GetProviderType()
}

// HealthCheck verifies connectivity to the storage
func (s *Storage) HealthCheck(ctx context.Context) error {
	return s.provider.HealthCheck(ctx)
}

// String returns a string representation of the storage
func (s *Storage) String() string {
	return fmt.Sprintf("%s - bucket: %s, endpoint: %s",
		s.provider.GetProviderType(), s.config.Bucket, s.config.Endpoint)
}

// IsLocalStorage returns true if config uses local-only storage
func IsLocalStorage(cfg *config.Config) bool {
	return cfg.IsLocal()
}
