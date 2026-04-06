package backup

import (
	"context"
	"io"
)

// Dumper defines the interface for database dumpers
type Dumper interface {
	// Dump creates a database dump and writes it to the provided writer
	Dump(ctx context.Context, w io.Writer, config DumpConfig) error
}

// DumpConfig holds configuration for a database dump
type DumpConfig struct {
	Type        string // mysql, postgres, mongo
	ContainerID string // optional when DSN is set
	DSN         string // direct connection string (e.g. postgres://user:pass@host/db)
	Database    string
	User        string
	Password    string
	Host        string
	Port        int
}
