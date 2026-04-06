package backup

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os/exec"
	"strconv"
)

// MongoDBDumper implements the Dumper interface for MongoDB databases
type MongoDBDumper struct{}

// NewMongoDBDumper creates a new MongoDB dumper
func NewMongoDBDumper() (*MongoDBDumper, error) {
	return &MongoDBDumper{}, nil
}

// Dump creates a MongoDB database dump
func (d *MongoDBDumper) Dump(ctx context.Context, w io.Writer, config DumpConfig) error {
	var cmd *exec.Cmd
	var stderrBuf bytes.Buffer

	if config.ContainerID != "" {
		// Docker path: exec mongodump inside container
		authDB := config.Database
		if config.User == "root" {
			authDB = "admin"
		}

		args := []string{
			"exec", "-i", config.ContainerID,
			"mongodump",
			"--username", config.User,
			"--password", config.Password,
			"--authenticationDatabase", authDB,
			"--archive",
		}

		if config.Database != "" && config.Database != "admin" {
			args = append(args, "--db", config.Database)
		}

		if config.Host != "" {
			args = append(args, "--host", config.Host)
		}
		if config.Port > 0 {
			args = append(args, "--port", strconv.Itoa(config.Port))
		}

		cmd = exec.CommandContext(ctx, "docker", args...)
	} else {
		// Direct path: call local mongodump
		authDB := config.Database
		if config.User == "root" {
			authDB = "admin"
		}

		args := []string{
			"--username", config.User,
			"--password", config.Password,
			"--authenticationDatabase", authDB,
			"--archive",
		}

		if config.Database != "" && config.Database != "admin" {
			args = append(args, "--db", config.Database)
		}

		if config.Host != "" {
			args = append(args, "--host", config.Host)
		}
		if config.Port > 0 {
			args = append(args, "--port", strconv.Itoa(config.Port))
		}

		cmd = exec.CommandContext(ctx, "mongodump", args...)
	}

	cmd.Stdout = w
	cmd.Stderr = &stderrBuf

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("mongodump failed: %w\nstderr: %s", err, stderrBuf.String())
	}

	return nil
}
