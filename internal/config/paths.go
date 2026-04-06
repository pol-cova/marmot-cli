package config

import (
	"os"
	"path/filepath"
	"runtime"
)

// Paths holds default paths for different platforms
type Paths struct {
	ConfigDir   string
	ConfigFile  string
	KeyFile     string
	StateDB     string
	BackupsDir  string
	LogFile     string
}

// canWrite checks if a directory is writable
func canWrite(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	if !info.IsDir() {
		return false
	}
	// Try to create a temp file to test write permissions
	testFile := filepath.Join(path, ".marmot-write-test")
	f, err := os.Create(testFile)
	if err != nil {
		return false
	}
	f.Close()
	os.Remove(testFile)
	return true
}

// fileExists checks if a file or directory exists (regardless of permissions)
func fileExists(path string) bool {
	_, err := os.Stat(path)
	return !os.IsNotExist(err)
}

// GetDefaultPaths returns platform-specific default paths
func GetDefaultPaths() *Paths {
	homeDir, _ := os.UserHomeDir()
	
	var configDir, stateDB, backupsDir, logFile string
	
	// Determine OS and set paths accordingly
	switch {
	case isLinux():
		// Check if system config exists first
		systemConfigFile := "/etc/marmot/config.yaml"
		if fileExists(systemConfigFile) {
			// System config exists, use system paths (user may need sudo to read)
			configDir = "/etc/marmot"
			stateDB = "/var/lib/marmot/state.db"
			backupsDir = "/var/lib/marmot/backups"
			logFile = "/var/log/marmot/marmot.log"
		} else if canWrite("/etc") {
			// System config doesn't exist but we can write to /etc, use system paths
			configDir = "/etc/marmot"
			stateDB = "/var/lib/marmot/state.db"
			backupsDir = "/var/lib/marmot/backups"
			logFile = "/var/log/marmot/marmot.log"
		} else {
			// Fallback to home directory for non-root users
			configDir = filepath.Join(homeDir, ".marmot")
			stateDB = filepath.Join(homeDir, ".marmot", "state.db")
			backupsDir = filepath.Join(homeDir, ".marmot", "backups")
			logFile = filepath.Join(homeDir, ".marmot", "marmot.log")
		}
	case isDarwin():
		configDir = filepath.Join(homeDir, ".marmot")
		stateDB = filepath.Join(homeDir, ".marmot", "state.db")
		backupsDir = filepath.Join(homeDir, ".marmot", "backups")
		logFile = filepath.Join(homeDir, ".marmot", "marmot.log")
	case isWindows():
		appData := os.Getenv("APPDATA")
		if appData == "" {
			appData = filepath.Join(homeDir, "AppData", "Roaming")
		}
		configDir = filepath.Join(appData, "marmot")
		stateDB = filepath.Join(appData, "marmot", "state.db")
		backupsDir = filepath.Join(appData, "marmot", "backups")
		logFile = filepath.Join(appData, "marmot", "marmot.log")
	default:
		// Fallback to home directory
		configDir = filepath.Join(homeDir, ".marmot")
		stateDB = filepath.Join(homeDir, ".marmot", "state.db")
		backupsDir = filepath.Join(homeDir, ".marmot", "backups")
		logFile = filepath.Join(homeDir, ".marmot", "marmot.log")
	}
	
	return &Paths{
		ConfigDir:  configDir,
		ConfigFile: filepath.Join(configDir, "config.yaml"),
		KeyFile:    filepath.Join(configDir, "key.bin"),
		StateDB:    stateDB,
		BackupsDir: backupsDir,
		LogFile:    logFile,
	}
}

func isLinux() bool {
	return runtime.GOOS == "linux"
}

func isDarwin() bool {
	return runtime.GOOS == "darwin"
}

func isWindows() bool {
	return runtime.GOOS == "windows"
}

