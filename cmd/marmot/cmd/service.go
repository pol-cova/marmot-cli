package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"text/template"

	"github.com/spf13/cobra"
)

const systemdServiceUnit = `[Unit]
Description=Marmot Database Backup Daemon
After=network.target
Wants=network-online.target

[Service]
Type=simple
ExecStart=/usr/local/bin/marmot start --foreground
Restart=on-failure
RestartSec=10
StandardOutput=journal
StandardError=journal
NoNewPrivileges=true
PrivateTmp=true

[Install]
WantedBy=multi-user.target
`

const launchdPlistTemplate = `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN"
  "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>dev.marmot.agent</string>
    <key>ProgramArguments</key>
    <array>
        <string>/usr/local/bin/marmot</string>
        <string>start</string>
        <string>--foreground</string>
    </array>
    <key>RunAtLoad</key>
    <true/>
    <key>KeepAlive</key>
    <true/>
    <key>StandardOutPath</key>
    <string>{{.LogDir}}/marmot.log</string>
    <key>StandardErrorPath</key>
    <string>{{.LogDir}}/marmot-error.log</string>
</dict>
</plist>
`

func newServiceCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "service",
		Short: "Manage the Marmot background service",
		Long:  "Install or uninstall the Marmot daemon as a system service (systemd on Linux, launchd on macOS)",
	}
	cmd.AddCommand(newServiceInstallCmd())
	cmd.AddCommand(newServiceUninstallCmd())
	return cmd
}

func newServiceInstallCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "install",
		Short: "Install and start Marmot as a system service",
		RunE: func(cmd *cobra.Command, args []string) error {
			switch runtime.GOOS {
			case "linux":
				return installSystemd()
			case "darwin":
				return installLaunchd()
			default:
				return fmt.Errorf("service install is not supported on %s", runtime.GOOS)
			}
		},
	}
}

func newServiceUninstallCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "uninstall",
		Short: "Stop and uninstall the Marmot system service",
		RunE: func(cmd *cobra.Command, args []string) error {
			switch runtime.GOOS {
			case "linux":
				return uninstallSystemd()
			case "darwin":
				return uninstallLaunchd()
			default:
				return fmt.Errorf("service uninstall is not supported on %s", runtime.GOOS)
			}
		},
	}
}

func installSystemd() error {
	servicePath := "/etc/systemd/system/marmot.service"
	if err := os.WriteFile(servicePath, []byte(systemdServiceUnit), 0644); err != nil {
		return fmt.Errorf("failed to write service file (try running as root): %w", err)
	}
	fmt.Printf("Service file written to: %s\n", servicePath)

	for _, args := range [][]string{
		{"daemon-reload"},
		{"enable", "marmot"},
		{"start", "marmot"},
	} {
		out, err := exec.Command("systemctl", args...).CombinedOutput()
		if err != nil {
			return fmt.Errorf("systemctl %v failed: %w\n%s", args, err, out)
		}
	}

	fmt.Println("Marmot service installed and started.")
	fmt.Println("Check status: systemctl status marmot")
	fmt.Println("View logs:    journalctl -u marmot -f")
	return nil
}

func uninstallSystemd() error {
	for _, args := range [][]string{
		{"stop", "marmot"},
		{"disable", "marmot"},
	} {
		out, err := exec.Command("systemctl", args...).CombinedOutput()
		if err != nil {
			fmt.Printf("Warning: systemctl %v failed: %v\n%s\n", args, err, out)
		}
	}

	servicePath := "/etc/systemd/system/marmot.service"
	if err := os.Remove(servicePath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove service file: %w", err)
	}

	if out, err := exec.Command("systemctl", "daemon-reload").CombinedOutput(); err != nil {
		fmt.Printf("Warning: daemon-reload failed: %v\n%s\n", err, out)
	}

	fmt.Println("Marmot service uninstalled.")
	return nil
}

func installLaunchd() error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get home directory: %w", err)
	}

	launchAgentsDir := filepath.Join(home, "Library", "LaunchAgents")
	if err := os.MkdirAll(launchAgentsDir, 0755); err != nil {
		return fmt.Errorf("failed to create LaunchAgents dir: %w", err)
	}

	logDir := filepath.Join(home, ".marmot")
	if err := os.MkdirAll(logDir, 0700); err != nil {
		return fmt.Errorf("failed to create log dir: %w", err)
	}

	plistPath := filepath.Join(launchAgentsDir, "dev.marmot.plist")
	f, err := os.Create(plistPath)
	if err != nil {
		return fmt.Errorf("failed to create plist file: %w", err)
	}
	defer f.Close()

	tmpl := template.Must(template.New("plist").Parse(launchdPlistTemplate))
	if err := tmpl.Execute(f, map[string]string{"LogDir": logDir}); err != nil {
		return fmt.Errorf("failed to write plist: %w", err)
	}
	fmt.Printf("LaunchAgent plist written to: %s\n", plistPath)

	out, err := exec.Command("launchctl", "load", plistPath).CombinedOutput()
	if err != nil {
		return fmt.Errorf("launchctl load failed: %w\n%s", err, out)
	}

	fmt.Println("Marmot LaunchAgent installed and started.")
	fmt.Printf("Logs: %s/marmot.log\n", logDir)
	return nil
}

func uninstallLaunchd() error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get home directory: %w", err)
	}

	plistPath := filepath.Join(home, "Library", "LaunchAgents", "dev.marmot.plist")
	out, err := exec.Command("launchctl", "unload", plistPath).CombinedOutput()
	if err != nil {
		fmt.Printf("Warning: launchctl unload failed: %v\n%s\n", err, out)
	}

	if err := os.Remove(plistPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove plist: %w", err)
	}

	fmt.Println("Marmot LaunchAgent uninstalled.")
	return nil
}
