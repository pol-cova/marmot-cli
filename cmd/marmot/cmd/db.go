package cmd

import (
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
	"text/tabwriter"

	"github.com/pol-cova/marmot-cli/internal/config"
	"github.com/spf13/cobra"
)

func newDbCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "db",
		Short: "Manage database configurations",
		Long:  "Add, list, or remove databases from the Marmot configuration",
	}
	cmd.AddCommand(newDbAddCmd())
	cmd.AddCommand(newDbListCmd())
	cmd.AddCommand(newDbRemoveCmd())
	return cmd
}

func newDbAddCmd() *cobra.Command {
	var (
		dbType      string
		dbID        string
		dsn         string
		containerID string
		dbName      string
		dbUser      string
		dbPassword  string
		dbHost      string
		dbPort      int
		schedule    string
	)

	cmd := &cobra.Command{
		Use:   "add",
		Short: "Add a database to the configuration",
		Long: `Add a database for Marmot to back up.

Docker mode (database running in a container):
  marmot db add --type postgres --id prod-pg --container mycontainer --name mydb --user app --password secret
  marmot db add --type mongo --id prod-mongo --container mongo-container --name mydb --user root --password secret

Direct mode (database running directly on the host):
  marmot db add --type postgres --id prod-pg --dsn "postgres://user:pass@localhost:5432/mydb"
  marmot db add --type mysql    --id prod-mysql --dsn "mysql://user:pass@localhost:3306/mydb"
  marmot db add --type mongo    --id prod-mongo --dsn "mongodb://user:pass@localhost:27017/mydb"`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if dbType == "" {
				return fmt.Errorf("--type is required (mysql, postgres, or mongo)")
			}
			if dbType != "mysql" && dbType != "postgres" && dbType != "mongo" {
				return fmt.Errorf("--type must be 'mysql', 'postgres', or 'mongo'")
			}
			if dbID == "" {
				return fmt.Errorf("--id is required")
			}
			if dsn == "" && containerID == "" {
				return fmt.Errorf("either --dsn or --container is required")
			}

			cfg, err := config.LoadConfig(getConfigPath())
			if err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}

			// Check for duplicate ID
			if cfg.GetDatabaseByID(dbID) != nil {
				return fmt.Errorf("database with id '%s' already exists", dbID)
			}

			dbConfig := config.DatabaseConfig{
				ID:          dbID,
				Type:        dbType,
				ContainerID: containerID,
				DSN:         dsn,
				Schedule:    schedule,
				Enabled:     true,
			}

			// Parse DSN to fill in connection fields if provided
			if dsn != "" {
				parsed, err := parseDSN(dsn, dbType)
				if err != nil {
					return fmt.Errorf("failed to parse DSN: %w", err)
				}
				dbConfig.Name = parsed.name
				dbConfig.User = parsed.user
				dbConfig.Password = parsed.password
				dbConfig.Host = parsed.host
				dbConfig.Port = parsed.port
			} else {
				// Docker mode: use explicit flags
				if dbName == "" {
					return fmt.Errorf("--name is required in Docker mode")
				}
				dbConfig.Name = dbName
				dbConfig.User = dbUser
				dbConfig.Password = dbPassword
				dbConfig.Host = dbHost
				dbConfig.Port = dbPort
			}

			cfg.Databases = append(cfg.Databases, dbConfig)
			if err := config.SaveConfig(cfg, cfg.Paths.ConfigFile); err != nil {
				return fmt.Errorf("failed to save config: %w", err)
			}

			conn := dbConfig.ContainerID
			if conn == "" {
				conn = dsn
			}
			fmt.Printf("Added database: %s (%s) — %s\n", dbID, dbType, conn)
			fmt.Printf("Schedule: %s\n", schedule)
			return nil
		},
	}

	cmd.Flags().StringVar(&dbType, "type", "", "database type: mysql or postgres (required)")
	cmd.Flags().StringVar(&dbID, "id", "", "unique identifier for this database (required)")
	cmd.Flags().StringVar(&dsn, "dsn", "", "connection string, e.g. postgres://user:pass@host:5432/db")
	cmd.Flags().StringVar(&containerID, "container", "", "Docker container ID or name")
	cmd.Flags().StringVar(&dbName, "name", "", "database name (Docker mode)")
	cmd.Flags().StringVar(&dbUser, "user", "", "database user (Docker mode)")
	cmd.Flags().StringVar(&dbPassword, "password", "", "database password (Docker mode)")
	cmd.Flags().StringVar(&dbHost, "host", "localhost", "database host (Docker mode)")
	cmd.Flags().IntVar(&dbPort, "port", 0, "database port (Docker mode)")
	cmd.Flags().StringVar(&schedule, "schedule", "0 2 * * *", "cron schedule for automatic backups")

	return cmd
}

type parsedDSN struct {
	user, password, host, name string
	port                       int
}

func parseDSN(dsn, dbType string) (*parsedDSN, error) {
	// Normalize mysql:// to a parseable URL scheme
	normalized := dsn
	if strings.HasPrefix(dsn, "mysql://") {
		normalized = strings.Replace(dsn, "mysql://", "http://", 1)
	} else if strings.HasPrefix(dsn, "postgres://") || strings.HasPrefix(dsn, "postgresql://") {
		normalized = strings.Replace(dsn, "postgresql://", "http://", 1)
		normalized = strings.Replace(normalized, "postgres://", "http://", 1)
	} else if strings.HasPrefix(dsn, "mongodb://") || strings.HasPrefix(dsn, "mongodb+srv://") {
		normalized = strings.Replace(dsn, "mongodb+srv://", "http://", 1)
		normalized = strings.Replace(normalized, "mongodb://", "http://", 1)
	}

	u, err := url.Parse(normalized)
	if err != nil {
		return nil, fmt.Errorf("invalid DSN format: %w", err)
	}

	result := &parsedDSN{
		host: u.Hostname(),
		name: strings.TrimPrefix(u.Path, "/"),
	}

	if u.User != nil {
		result.user = u.User.Username()
		result.password, _ = u.User.Password()
	}

	if portStr := u.Port(); portStr != "" {
		result.port, _ = strconv.Atoi(portStr)
	} else {
		switch dbType {
		case "postgres":
			result.port = 5432
		case "mysql":
			result.port = 3306
		case "mongo":
			result.port = 27017
		}
	}

	if result.host == "" {
		result.host = "localhost"
	}

	return result, nil
}

func newDbListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List configured databases",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.LoadConfig(getConfigPath())
			if err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}

			if len(cfg.Databases) == 0 {
				fmt.Println("No databases configured.")
				fmt.Println("Add one with: marmot db add --type postgres --dsn \"postgres://...\" --id mydb")
				return nil
			}

			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "ID\tTYPE\tCONNECTION\tSCHEDULE\tENABLED")
			fmt.Fprintln(w, "──\t────\t──────────\t────────\t───────")
			for _, db := range cfg.Databases {
				conn := db.ContainerID
				if conn == "" && db.DSN != "" {
					conn = db.DSN
				}
				if len(conn) > 45 {
					conn = conn[:42] + "..."
				}
				enabled := "yes"
				if !db.Enabled {
					enabled = "no"
				}
				fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", db.ID, db.Type, conn, db.Schedule, enabled)
			}
			w.Flush()
			return nil
		},
	}
}

func newDbRemoveCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "remove <id>",
		Short: "Remove a database from configuration",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id := args[0]
			cfg, err := config.LoadConfig(getConfigPath())
			if err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}

			newDbs := cfg.Databases[:0]
			found := false
			for _, db := range cfg.Databases {
				if db.ID == id {
					found = true
				} else {
					newDbs = append(newDbs, db)
				}
			}
			if !found {
				return fmt.Errorf("database not found: %s", id)
			}

			cfg.Databases = newDbs
			if err := config.SaveConfig(cfg, cfg.Paths.ConfigFile); err != nil {
				return fmt.Errorf("failed to save config: %w", err)
			}

			fmt.Printf("Removed database: %s\n", id)
			return nil
		},
	}
}
