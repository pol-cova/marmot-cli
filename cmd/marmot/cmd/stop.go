package cmd

import (
	"fmt"
	"os"
	"time"

	"github.com/pol-cova/marmot-cli/internal/config"
	"github.com/pol-cova/marmot-cli/internal/daemon"

	"github.com/spf13/cobra"
)

func newStopCmd() *cobra.Command {
	var force bool

	cmd := &cobra.Command{
		Use:   "stop",
		Short: "Stop the Marmot daemon",
		Long:  "Stops the Marmot daemon process used for scheduled backups",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runStopDaemon(force)
		},
	}

	cmd.Flags().BoolVar(&force, "force", false, "use shorter shutdown timeout")
	return cmd
}

func runStopDaemon(force bool) error {
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
}
