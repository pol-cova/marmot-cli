package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"time"

	"github.com/pol-cova/marmot-cli/internal/config"
	"github.com/pol-cova/marmot-cli/internal/daemon"
)

func launchDaemonBackground(cfg *config.Config) error {
	exePath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to locate executable: %w", err)
	}

	args := []string{"start", "--foreground"}
	if configPath != "" {
		args = append(args, "--config", configPath)
	}

	if err := os.MkdirAll(filepath.Dir(cfg.Paths.LogFile), 0750); err != nil {
		return fmt.Errorf("failed to create log directory: %w", err)
	}

	logFile, err := os.OpenFile(cfg.Paths.LogFile, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0640)
	if err != nil {
		return fmt.Errorf("failed to open log file: %w", err)
	}
	defer logFile.Close()

	cmd := exec.Command(exePath, args...)
	cmd.Stdin = nil
	cmd.Stdout = logFile
	cmd.Stderr = logFile

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to launch daemon process: %w", err)
	}

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		pid, err := daemon.ReadPIDFile(cfg.Paths.PIDFile)
		if err == nil && daemon.IsProcessRunning(pid) {
			if relErr := cmd.Process.Release(); relErr != nil {
				return fmt.Errorf("failed to release daemon process handle: %w", relErr)
			}
			return nil
		}

		if signalErr := cmd.Process.Signal(syscall.Signal(0)); signalErr != nil {
			return fmt.Errorf("daemon exited before startup completed")
		}

		time.Sleep(100 * time.Millisecond)
	}

	if relErr := cmd.Process.Release(); relErr != nil {
		return fmt.Errorf("failed to release daemon process handle: %w", relErr)
	}

	return nil
}
