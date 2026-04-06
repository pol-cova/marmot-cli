package daemon

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestAcquirePIDFileAndRelease(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	pidPath := filepath.Join(dir, "marmot.pid")

	release, err := AcquirePIDFile(pidPath)
	if err != nil {
		t.Fatalf("AcquirePIDFile() error = %v", err)
	}

	pid, err := ReadPIDFile(pidPath)
	if err != nil {
		t.Fatalf("ReadPIDFile() error = %v", err)
	}
	if pid != os.Getpid() {
		t.Fatalf("ReadPIDFile() pid = %d, want %d", pid, os.Getpid())
	}

	release()
	if _, err := os.Stat(pidPath); !os.IsNotExist(err) {
		t.Fatalf("pid file should be removed, stat err = %v", err)
	}
}

func TestAcquirePIDFileDetectsAlreadyRunning(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	pidPath := filepath.Join(dir, "marmot.pid")

	if err := os.WriteFile(pidPath, []byte("1\n"), 0600); err != nil {
		t.Fatalf("os.WriteFile() error = %v", err)
	}

	_, err := AcquirePIDFile(pidPath)
	if err == nil {
		t.Fatal("AcquirePIDFile() expected already running error")
	}

	if _, ok := err.(*AlreadyRunningError); !ok {
		t.Fatalf("AcquirePIDFile() error type = %T, want *AlreadyRunningError", err)
	}
}

func TestAcquirePIDFileReplacesStalePID(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	pidPath := filepath.Join(dir, "marmot.pid")

	if err := os.WriteFile(pidPath, []byte("999999\n"), 0600); err != nil {
		t.Fatalf("os.WriteFile() error = %v", err)
	}

	release, err := AcquirePIDFile(pidPath)
	if err != nil {
		t.Fatalf("AcquirePIDFile() error = %v", err)
	}
	release()
}

func TestStopProcessInvalidPID(t *testing.T) {
	t.Parallel()

	err := StopProcess(-1, time.Second)
	if err == nil {
		t.Fatal("StopProcess() expected error for invalid pid")
	}
}
