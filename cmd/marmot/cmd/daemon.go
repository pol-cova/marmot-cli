package cmd

import (
	"fmt"
	"os"
	"time"

	"github.com/pol-cova/marmot-cli/internal/config"
	"github.com/pol-cova/marmot-cli/internal/daemon"

	"github.com/spf13/cobra"
)

func newDaemonCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "daemon",
		Short: "Manage Marmot daemon process",
		Long:  "Start, stop, restart, and inspect the Marmot daemon process",
	}

	cmd.AddCommand(newDaemonStatusCmd())
	cmd.AddCommand(newDaemonStopCmd())
	cmd.AddCommand(newDaemonRestartCmd())

	return cmd
}

func newDaemonStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show daemon process status",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.LoadConfig(getConfigPath())
			if err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}

			pid, err := daemon.ReadPIDFile(cfg.Paths.PIDFile)
			if err != nil {
				if os.IsNotExist(err) {
					fmt.Println("Daemon status: not running")
					fmt.Printf("PID file: %s (not found)\n", cfg.Paths.PIDFile)
					return nil
				}
				fmt.Printf("Daemon status: unknown\n")
				fmt.Printf("PID file: %s (invalid: %v)\n", cfg.Paths.PIDFile, err)
				return nil
			}

			if daemon.IsProcessRunning(pid) {
				fmt.Printf("Daemon status: running (pid %d)\n", pid)
				fmt.Printf("PID file: %s\n", cfg.Paths.PIDFile)
				fmt.Printf("Log file: %s\n", cfg.Paths.LogFile)
				return nil
			}

			fmt.Printf("Daemon status: stale pid file (pid %d not running)\n", pid)
			fmt.Printf("PID file: %s\n", cfg.Paths.PIDFile)
			return nil
		},
	}
}

func newDaemonStopCmd() *cobra.Command {
	var force bool

	cmd := &cobra.Command{
		Use:   "stop",
		Short: "Stop daemon process",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.LoadConfig(getConfigPath())
			if err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}

			pid, err := daemon.ReadPIDFile(cfg.Paths.PIDFile)
			if err != nil {
				if os.IsNotExist(err) {
					fmt.Println("Daemon is not running.")
					return nil
				}
				return fmt.Errorf("failed to read pid file: %w", err)
			}

			if !daemon.IsProcessRunning(pid) {
				_ = os.Remove(cfg.Paths.PIDFile)
				fmt.Printf("Removed stale pid file: %s\n", cfg.Paths.PIDFile)
				return nil
			}

			timeout := 10 * time.Second
			if force {
				timeout = 2 * time.Second
			}

			if err := daemon.StopProcess(pid, timeout); err != nil {
				return err
			}

			_ = os.Remove(cfg.Paths.PIDFile)
			fmt.Printf("Stopped daemon (pid %d).\n", pid)
			return nil
		},
	}

	cmd.Flags().BoolVar(&force, "force", false, "use shorter shutdown timeout")
	return cmd
}

func newDaemonRestartCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "restart",
		Short: "Restart daemon process",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.LoadConfig(getConfigPath())
			if err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}

			pid, err := daemon.ReadPIDFile(cfg.Paths.PIDFile)
			if err == nil && daemon.IsProcessRunning(pid) {
				if err := daemon.StopProcess(pid, 10*time.Second); err != nil {
					return err
				}
				_ = os.Remove(cfg.Paths.PIDFile)
				fmt.Printf("Stopped daemon (pid %d).\n", pid)
			}

			if err := launchDaemonBackground(cfg); err != nil {
				return err
			}

			newPID, _ := daemon.ReadPIDFile(cfg.Paths.PIDFile)
			fmt.Printf("Started daemon (pid %d).\n", newPID)
			return nil
		},
	}
}
