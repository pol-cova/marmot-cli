package cmd

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/pol-cova/marmot-cli/internal/config"
)

func verifyDiscoveredConnection(ctx context.Context, db config.DatabaseConfig) (string, error) {
	verifyCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	switch db.Type {
	case "postgres":
		return verifyPostgresConnection(verifyCtx, db)
	case "mysql":
		return verifyMySQLConnection(verifyCtx, db)
	case "mongo":
		return verifyMongoConnection(verifyCtx, db)
	default:
		return "unsupported", fmt.Errorf("unsupported database type: %s", db.Type)
	}
}

func verifyPostgresConnection(ctx context.Context, db config.DatabaseConfig) (string, error) {
	var cmd *exec.Cmd
	var stderr bytes.Buffer

	args := []string{"pg_dump", "-s", "-U", db.User}
	if db.Host != "" {
		args = append(args, "-h", db.Host)
	}
	if db.Port > 0 {
		args = append(args, "-p", strconv.Itoa(db.Port))
	}
	args = append(args, db.Name)

	if db.ContainerID != "" {
		if _, err := exec.LookPath("docker"); err != nil {
			return "", fmt.Errorf("docker not found")
		}
		dockerArgs := append([]string{"exec", "-i", "-e", "PGPASSWORD", db.ContainerID}, args...)
		cmd = exec.CommandContext(ctx, "docker", dockerArgs...)
	} else {
		if _, err := exec.LookPath("pg_dump"); err != nil {
			return "", fmt.Errorf("pg_dump not found")
		}
		cmd = exec.CommandContext(ctx, args[0], args[1:]...)
	}

	cmd.Env = append(os.Environ(), "PGPASSWORD="+db.Password)
	cmd.Stdout = io.Discard
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("pg_dump failed: %w; %s", err, strings.TrimSpace(stderr.String()))
	}

	return "authenticated", nil
}

func verifyMySQLConnection(ctx context.Context, db config.DatabaseConfig) (string, error) {
	var cmd *exec.Cmd
	var stderr bytes.Buffer

	args := []string{"mysqldump", "--no-data", "-u", db.User}
	if db.Host != "" {
		args = append(args, "-h", db.Host)
	}
	if db.Port > 0 {
		args = append(args, "-P", strconv.Itoa(db.Port))
	}
	args = append(args, db.Name)

	if db.ContainerID != "" {
		if _, err := exec.LookPath("docker"); err != nil {
			return "", fmt.Errorf("docker not found")
		}
		dockerArgs := append([]string{"exec", "-i", "-e", "MYSQL_PWD", db.ContainerID}, args...)
		cmd = exec.CommandContext(ctx, "docker", dockerArgs...)
	} else {
		if _, err := exec.LookPath("mysqldump"); err != nil {
			return "", fmt.Errorf("mysqldump not found")
		}
		cmd = exec.CommandContext(ctx, args[0], args[1:]...)
	}

	cmd.Env = append(os.Environ(), "MYSQL_PWD="+db.Password)
	cmd.Stdout = io.Discard
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("mysqldump failed: %w; %s", err, strings.TrimSpace(stderr.String()))
	}

	return "authenticated", nil
}

func verifyMongoConnection(ctx context.Context, db config.DatabaseConfig) (string, error) {
	args := []string{"mongosh", "--quiet", "--host", defaultString(db.Host, "localhost")}
	if db.Port > 0 {
		args = append(args, "--port", strconv.Itoa(db.Port))
	}
	if db.User != "" {
		args = append(args, "--username", db.User)
	}
	if db.Password != "" {
		args = append(args, "--password", db.Password)
	}
	authDB := db.Name
	if db.User == "root" || authDB == "" {
		authDB = "admin"
	}
	args = append(args, "--authenticationDatabase", authDB, "--eval", "db.runCommand({ ping: 1 })")

	var cmd *exec.Cmd
	if db.ContainerID != "" {
		if _, err := exec.LookPath("docker"); err != nil {
			return "", fmt.Errorf("docker not found")
		}
		dockerArgs := append([]string{"exec", "-i", db.ContainerID}, args...)
		cmd = exec.CommandContext(ctx, "docker", dockerArgs...)
	} else {
		if _, err := exec.LookPath("mongosh"); err != nil {
			return "", fmt.Errorf("mongosh not found (install MongoDB Shell to verify auth)")
		}
		cmd = exec.CommandContext(ctx, args[0], args[1:]...)
	}

	var stderr bytes.Buffer
	cmd.Stdout = io.Discard
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("mongosh failed: %w; %s", err, strings.TrimSpace(stderr.String()))
	}

	return "authenticated", nil
}
