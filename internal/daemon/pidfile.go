package daemon

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"
)

type AlreadyRunningError struct {
	PID int
}

func (e *AlreadyRunningError) Error() string {
	return fmt.Sprintf("daemon already running (pid %d)", e.PID)
}

func AcquirePIDFile(pidFile string) (func(), error) {
	if err := os.MkdirAll(filepath.Dir(pidFile), 0700); err != nil {
		return nil, fmt.Errorf("failed to create pid directory: %w", err)
	}

	release, err := tryAcquire(pidFile)
	if err == nil {
		return release, nil
	}

	if !errors.Is(err, os.ErrExist) {
		return nil, err
	}

	pid, readErr := ReadPIDFile(pidFile)
	if readErr != nil {
		if rmErr := os.Remove(pidFile); rmErr != nil && !os.IsNotExist(rmErr) {
			return nil, fmt.Errorf("failed to remove invalid pid file: %w", rmErr)
		}
		return tryAcquire(pidFile)
	}

	if IsProcessRunning(pid) {
		return nil, &AlreadyRunningError{PID: pid}
	}

	if rmErr := os.Remove(pidFile); rmErr != nil && !os.IsNotExist(rmErr) {
		return nil, fmt.Errorf("failed to remove stale pid file: %w", rmErr)
	}

	return tryAcquire(pidFile)
}

func tryAcquire(pidFile string) (func(), error) {
	f, err := os.OpenFile(pidFile, os.O_RDWR|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		if os.IsExist(err) {
			return nil, os.ErrExist
		}
		return nil, fmt.Errorf("failed to create pid file: %w", err)
	}

	pid := os.Getpid()
	if _, err := fmt.Fprintf(f, "%d\n", pid); err != nil {
		_ = f.Close()
		_ = os.Remove(pidFile)
		return nil, fmt.Errorf("failed to write pid file: %w", err)
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(pidFile)
		return nil, fmt.Errorf("failed to close pid file: %w", err)
	}

	released := false
	release := func() {
		if released {
			return
		}
		released = true
		_ = os.Remove(pidFile)
	}

	return release, nil
}

func ReadPIDFile(pidFile string) (int, error) {
	data, err := os.ReadFile(pidFile)
	if err != nil {
		return 0, err
	}

	pidStr := strings.TrimSpace(string(data))
	if pidStr == "" {
		return 0, fmt.Errorf("pid file is empty")
	}

	pid, err := strconv.Atoi(pidStr)
	if err != nil || pid <= 0 {
		return 0, fmt.Errorf("invalid pid value: %q", pidStr)
	}

	return pid, nil
}

func IsProcessRunning(pid int) bool {
	if pid <= 0 {
		return false
	}

	proc, err := os.FindProcess(pid)
	if err != nil {
		return false
	}

	err = proc.Signal(syscall.Signal(0))
	return err == nil || errors.Is(err, syscall.EPERM)
}

func StopProcess(pid int, timeout time.Duration) error {
	if pid <= 0 {
		return fmt.Errorf("invalid pid: %d", pid)
	}

	proc, err := os.FindProcess(pid)
	if err != nil {
		return fmt.Errorf("failed to find process %d: %w", pid, err)
	}

	if runtime.GOOS == "windows" {
		if err := proc.Signal(syscall.SIGTERM); err != nil {
			if errors.Is(err, os.ErrProcessDone) {
				return nil
			}
			return fmt.Errorf("failed to signal pid %d: %w", pid, err)
		}
	} else if err := proc.Signal(syscall.SIGTERM); err != nil {
		if errors.Is(err, os.ErrProcessDone) {
			return nil
		}
		return fmt.Errorf("failed to send SIGTERM to pid %d: %w", pid, err)
	}

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if !IsProcessRunning(pid) {
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}

	return fmt.Errorf("timed out waiting for pid %d to stop", pid)
}
