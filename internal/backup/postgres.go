package backup

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
)

// PostgreSQLDumper implements the Dumper interface for PostgreSQL databases
type PostgreSQLDumper struct{}

// NewPostgreSQLDumper creates a new PostgreSQL dumper
func NewPostgreSQLDumper() (*PostgreSQLDumper, error) {
	return &PostgreSQLDumper{}, nil
}

// Dump creates a PostgreSQL database dump
func (d *PostgreSQLDumper) Dump(ctx context.Context, w io.Writer, config DumpConfig) error {
	var cmd *exec.Cmd
	var stderrBuf bytes.Buffer

	if config.ContainerID != "" {
		// Docker path: exec pg_dump inside container with PGPASSWORD env var
		args := []string{
			"exec", "-i",
			"-e", "PGPASSWORD=" + config.Password,
			config.ContainerID,
			"pg_dump",
			"-U", config.User,
		}
		if config.Host != "" {
			args = append(args, "-h", config.Host)
		}
		if config.Port > 0 {
			args = append(args, "-p", strconv.Itoa(config.Port))
		}
		args = append(args, "-F", "c", "-b", config.Database)
		cmd = exec.CommandContext(ctx, "docker", args...)
	} else {
		// Direct path: call local pg_dump with PGPASSWORD env var
		args := []string{"-U", config.User, "-F", "c", "-b"}
		if config.Host != "" {
			args = append(args, "-h", config.Host)
		}
		if config.Port > 0 {
			args = append(args, "-p", strconv.Itoa(config.Port))
		}
		args = append(args, config.Database)
		cmd = exec.CommandContext(ctx, "pg_dump", args...)
		cmd.Env = append(os.Environ(), "PGPASSWORD="+config.Password)
	}

	cmd.Stdout = w
	cmd.Stderr = &stderrBuf

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("pg_dump failed: %w\nstderr: %s", err, stderrBuf.String())
	}

	return nil
}

