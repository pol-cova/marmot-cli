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

// MySQLDumper implements the Dumper interface for MySQL databases
type MySQLDumper struct{}

// NewMySQLDumper creates a new MySQL dumper
func NewMySQLDumper() (*MySQLDumper, error) {
	return &MySQLDumper{}, nil
}

// Dump creates a MySQL database dump
func (d *MySQLDumper) Dump(ctx context.Context, w io.Writer, config DumpConfig) error {
	var cmd *exec.Cmd
	var stderrBuf bytes.Buffer

	if config.ContainerID != "" {
		// Docker path: exec mysqldump inside container
		// Pass MYSQL_PWD via env passthrough to avoid exposing password in process args.
		args := []string{
			"exec", "-i", config.ContainerID,
			"-e", "MYSQL_PWD",
			"mysqldump",
			"-u", config.User,
			"--single-transaction",
			"--routines",
			"--triggers",
			config.Database,
		}
		cmd = exec.CommandContext(ctx, "docker", args...)
		cmd.Env = append(os.Environ(), "MYSQL_PWD="+config.Password)
	} else {
		// Direct path: call local mysqldump with MYSQL_PWD env var
		args := []string{
			"--single-transaction",
			"--routines",
			"--triggers",
			"-u", config.User,
		}
		if config.Host != "" {
			args = append(args, "-h", config.Host)
		}
		if config.Port > 0 {
			args = append(args, "-P", strconv.Itoa(config.Port))
		}
		args = append(args, config.Database)
		cmd = exec.CommandContext(ctx, "mysqldump", args...)
		cmd.Env = append(os.Environ(), "MYSQL_PWD="+config.Password)
	}

	cmd.Stdout = w
	cmd.Stderr = &stderrBuf

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("mysqldump failed: %w\nstderr: %s", err, stderrBuf.String())
	}

	return nil
}
