package cmd

import (
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/pol-cova/marmot-cli/internal/config"
	"github.com/pol-cova/marmot-cli/internal/daemon"
	"github.com/pol-cova/marmot-cli/internal/remote"
	"github.com/pol-cova/marmot-cli/internal/storage"

	"github.com/spf13/cobra"
)

func newStatusCmd() *cobra.Command {
	var detailed bool

	cmd := &cobra.Command{
		Use:   "status",
		Short: "Show Marmot status",
		Long:  "Displays daemon status, storage connectivity, and database backup status",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runStatus(cmd, detailed)
		},
	}

	cmd.Flags().BoolVar(&detailed, "detailed", false, "show detailed information")

	return cmd
}

func runStatus(cmd *cobra.Command, detailed bool) error {
	cfg, err := config.LoadConfig(getConfigPath())
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	defer w.Flush()

	fmt.Fprintln(w, "=== Daemon Status ===")
	pid, pidErr := daemon.ReadPIDFile(cfg.Paths.PIDFile)
	if pidErr != nil {
		if os.IsNotExist(pidErr) {
			fmt.Fprintf(w, "Status:\tNot running\n")
		} else {
			fmt.Fprintf(w, "Status:\tUnknown (invalid pid file)\n")
		}
	} else if daemon.IsProcessRunning(pid) {
		fmt.Fprintf(w, "Status:\tRunning\n")
		fmt.Fprintf(w, "PID:\t%d\n", pid)
	} else {
		fmt.Fprintf(w, "Status:\tNot running (stale pid file)\n")
		fmt.Fprintf(w, "Last PID:\t%d\n", pid)
	}
	fmt.Fprintf(w, "PID file:\t%s\n", cfg.Paths.PIDFile)
	fmt.Fprintf(w, "Log file:\t%s\n", cfg.Paths.LogFile)
	fmt.Fprintln(w, "")

	// Storage Status
	fmt.Fprintln(w, "=== Storage Status ===")

	if cfg.IsLocal() {
		// Local storage status
		fmt.Fprintf(w, "Type:\tLocal Only\n")
		fmt.Fprintf(w, "Path:\t%s\n", cfg.GetStoragePath())

		if cfg.Local.RetentionDays > 0 {
			fmt.Fprintf(w, "Retention:\t%d days\n", cfg.Local.RetentionDays)
		} else {
			fmt.Fprintf(w, "Retention:\tunlimited\n")
		}

		if cfg.Local.MinFreeSpaceGB > 0 {
			fmt.Fprintf(w, "Min Free Space:\t%d GB\n", cfg.Local.MinFreeSpaceGB)
		}

		// Check local storage health
		stor, err := storage.NewLocalStorage(cfg.GetStoragePath())
		if err != nil {
			fmt.Fprintf(w, "Status:\tError\n")
			fmt.Fprintf(w, "Error:\t%v\n", err)
		} else {
			usage, err := stor.GetDiskUsage()
			if err != nil {
				fmt.Fprintf(w, "Status:\tError\n")
				fmt.Fprintf(w, "Error:\t%v\n", err)
			} else {
				fmt.Fprintf(w, "Status:\tReady\n")
				fmt.Fprintf(w, "Used:\t%d MB\n", usage.Used/(1024*1024))
				if usage.Total > 0 {
					fmt.Fprintf(w, "Total:\t%d MB\n", usage.Total/(1024*1024))
					fmt.Fprintf(w, "Available:\t%d MB\n", usage.Available/(1024*1024))
				}

				// Check free space warning
				if cfg.Local.MinFreeSpaceGB > 0 && usage.Available < int64(cfg.Local.MinFreeSpaceGB*1024*1024*1024) {
					fmt.Fprintf(w, "Warning:\tLow disk space!\n")
				}
			}

			files, err := stor.List()
			if err == nil {
				fmt.Fprintf(w, "Backup files:\t%d\n", len(files))
			}
		}
	} else {
		// S3/Cloud storage status
		fmt.Fprintf(w, "Type:\tCloud (%s)\n", cfg.S3.Provider)

		remoteStorage, err := remote.NewStorageWithContext(cmd.Context(), cfg)
		if err != nil {
			fmt.Fprintf(w, "Status:\tConfiguration Error\n")
			fmt.Fprintf(w, "Error:\t%v\n", err)
		} else {
			if err := remoteStorage.HealthCheck(cmd.Context()); err != nil {
				fmt.Fprintf(w, "Status:\tDisconnected\n")
				fmt.Fprintf(w, "Health:\tFailed\n")
				fmt.Fprintf(w, "Error:\t%v\n", err)
			} else {
				fmt.Fprintf(w, "Status:\tConnected\n")
				fmt.Fprintf(w, "Health:\tOK\n")
				fmt.Fprintf(w, "Provider:\t%s\n", remoteStorage.GetProviderType())
				fmt.Fprintf(w, "Bucket:\t%s\n", cfg.S3.Bucket)
				if cfg.S3.Endpoint != "" {
					fmt.Fprintf(w, "Endpoint:\t%s\n", cfg.S3.Endpoint)
				}
			}
		}

		// Also show local cache status
		fmt.Fprintln(w, "\n=== Local Cache ===")
		stor, err := storage.NewLocalStorage(cfg.Paths.BackupsDir)
		if err != nil {
			fmt.Fprintf(w, "Error:\t%v\n", err)
		} else {
			usage, err := stor.GetDiskUsage()
			if err != nil {
				fmt.Fprintf(w, "Error:\t%v\n", err)
			} else {
				fmt.Fprintf(w, "Used:\t%d MB\n", usage.Used/(1024*1024))
				if usage.Total > 0 {
					fmt.Fprintf(w, "Available:\t%d MB\n", usage.Available/(1024*1024))
				}
			}

			files, err := stor.List()
			if err == nil {
				fmt.Fprintf(w, "Cached files:\t%d\n", len(files))
			}
		}
	}

	// Upload queue (only relevant for cloud storage)
	if cfg.IsS3() {
		fmt.Fprintln(w, "\n=== Upload Queue ===")
		queue, err := storage.NewSQLiteQueue(cfg.Paths.StateDB)
		if err != nil {
			fmt.Fprintf(w, "Error:\t%v\n", err)
		} else {
			defer queue.Close()
			items, err := queue.List()
			if err != nil {
				fmt.Fprintf(w, "Error:\t%v\n", err)
			} else {
				fmt.Fprintf(w, "Queued items:\t%d\n", len(items))
			}
		}
	}

	// Databases
	fmt.Fprintln(w, "\n=== Databases ===")
	for _, db := range cfg.Databases {
		status := "Disabled"
		if db.Enabled {
			status = "Enabled"
		}

		fmt.Fprintf(w, "%s:\t%s\tSchedule: %s\n", db.ID, status, db.Schedule)

		if detailed {
			conn := db.ContainerID
			if len(conn) > 12 {
				conn = conn[:12]
			} else if conn == "" && db.DSN != "" {
				conn = "(direct)"
			}
			fmt.Fprintf(w, "  Type: %s, Connection: %s, Database: %s\n", db.Type, conn, db.Name)
		}
	}

	return nil
}
