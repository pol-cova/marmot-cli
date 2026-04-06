package cmd

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/pol-cova/marmot-cli/internal/config"
	"github.com/pol-cova/marmot-cli/internal/crypto"
	"github.com/pol-cova/marmot-cli/internal/discovery"
	"github.com/pol-cova/marmot-cli/internal/remote"
	"github.com/pol-cova/marmot-cli/internal/s3"

	"github.com/spf13/cobra"
)

func newInitCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "init",
		Short: "Initialize Marmot configuration",
		Long:  "Interactive setup wizard to configure Marmot for first-time use",
		RunE:  runInit,
	}

	return cmd
}

func runInit(cmd *cobra.Command, args []string) error {
	fmt.Println("Welcome to Marmot!")
	fmt.Println("This wizard will help you set up Marmot for the first time.")
	fmt.Println()

	reader := bufio.NewReader(os.Stdin)

	// STEP 1: Choose storage type (FIRST QUESTION)
	fmt.Println("=== Storage Type ===")
	fmt.Println()
	fmt.Println("Choose where you want to store your backups:")
	fmt.Println()

	storageType := promptSelect(reader, "Storage type", []string{"cloud", "local"})

	var cfg config.Config
	cfg.Paths = config.GetDefaultPaths()

	if storageType == "cloud" {
		// Cloud storage configuration
		if err := configureCloudStorage(reader, &cfg, cmd); err != nil {
			return err
		}
	} else {
		// Local storage configuration
		if err := configureLocalStorage(reader, &cfg); err != nil {
			return err
		}
	}

	// Docker discovery (same for both storage types)
	fmt.Println("\n=== Database Discovery ===")
	fmt.Println("Scanning Docker containers for databases...")

	var databases []discovery.DatabaseInfo
	discoverer, err := discovery.NewDockerDiscoverer()
	if err != nil {
		fmt.Printf("Warning: Docker not available (%v). Skipping auto-discovery.\n", err)
		fmt.Println("Use 'marmot db add' to configure databases manually.")
	} else {
		databases, err = discoverer.Discover(cmd.Context())
		if err != nil {
			fmt.Printf("Warning: Failed to discover databases: %v\n", err)
		}
	}

	if len(databases) == 0 {
		fmt.Println("No databases found in Docker containers.")
		if !confirm(reader, "Continue without databases?") {
			return fmt.Errorf("initialization cancelled")
		}
	}

	// Select databases
	var dbConfigs []config.DatabaseConfig
	for _, db := range databases {
		fmt.Printf("\nFound %s database: %s (container: %s)\n", db.Type, db.Database, db.ContainerName)
		if confirm(reader, "Add this database?") {
			schedule := prompt(reader, "Backup schedule (cron expression)", "0 2 * * *")

			dbConfig := config.DatabaseConfig{
				ID:          fmt.Sprintf("%s-%s", db.ContainerID[:12], db.Database),
				Type:        string(db.Type),
				ContainerID: db.ContainerID,
				Name:        db.Database,
				User:        db.User,
				Password:    db.Password,
				Host:        db.Host,
				Port:        db.Port,
				Schedule:    schedule,
				Enabled:     true,
			}

			dbConfigs = append(dbConfigs, dbConfig)
		}
	}

	cfg.Databases = dbConfigs

	// Generate encryption key (same for both storage types)
	fmt.Println("\n=== Encryption ===")
	fmt.Println("Generating encryption key...")

	encryptor := crypto.NewAESEncryptor()
	key, err := encryptor.GenerateKey()
	if err != nil {
		return fmt.Errorf("failed to generate encryption key: %w", err)
	}

	if err := os.MkdirAll(cfg.Paths.ConfigDir, 0700); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	encryptor.LoadKey(key)
	if err := encryptor.SaveKeyToFile(cfg.Paths.KeyFile); err != nil {
		return fmt.Errorf("failed to save encryption key: %w", err)
	}

	fmt.Printf("Encryption key saved to: %s\n", cfg.Paths.KeyFile)

	// Save config
	if err := config.SaveConfig(&cfg, cfg.Paths.ConfigFile); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	fmt.Printf("\nConfiguration saved to: %s\n", cfg.Paths.ConfigFile)
	fmt.Println("\nInitialization complete!")
	fmt.Println()

	// Show different next steps based on storage type
	if cfg.IsLocal() {
		fmt.Println("Storage: Local Only")
		fmt.Printf("Backup path: %s\n", cfg.GetStoragePath())
		if cfg.Local.RetentionDays > 0 {
			fmt.Printf("Retention: %d days\n", cfg.Local.RetentionDays)
		}
		fmt.Println()
		fmt.Println("Next steps:")
		fmt.Printf("  marmot backup --all          # Backup all databases\n")
		fmt.Printf("  marmot status                # Check daemon/storage status\n")
		fmt.Printf("  marmot cleanup               # Clean up old backups (if retention set)\n")
		fmt.Printf("  marmot service install       # Auto-start on reboot (recommended)\n")
	} else {
		fmt.Printf("Storage: Cloud (%s)\n", cfg.S3.Provider)
		fmt.Printf("Bucket: %s\n", cfg.S3.Bucket)
		fmt.Println()
		fmt.Println("Next steps:")
		fmt.Printf("  marmot backup --all          # Backup all databases\n")
		fmt.Printf("  marmot status                # Check daemon/storage status\n")
		fmt.Printf("  marmot service install       # Auto-start on reboot (recommended)\n")
	}

	fmt.Println()
	fmt.Println("Starting Marmot daemon in background...")
	if err := launchDaemonBackground(&cfg); err != nil {
		fmt.Printf("Warning: failed to auto-start daemon: %v\n", err)
		fmt.Println("Run 'marmot start' manually to start scheduled backups.")
	} else {
		fmt.Println("Daemon started.")
		fmt.Printf("PID file: %s\n", cfg.Paths.PIDFile)
		fmt.Printf("Logs: %s\n", cfg.Paths.LogFile)
	}

	fmt.Println()
	fmt.Println("╔══════════════════════════════════════════════════════════╗")
	fmt.Println("║  [!] IMPORTANT: Export and store your encryption key!     ║")
	fmt.Println("╚══════════════════════════════════════════════════════════╝")
	fmt.Println()
	fmt.Println("  Run:  marmot key export")
	fmt.Println()
	fmt.Println("  Save the output to a password manager or secure offline")
	fmt.Println("  location OUTSIDE this server. Without it, backups CANNOT")
	fmt.Println("  be decrypted if this server is lost.")

	return nil
}

func configureCloudStorage(reader *bufio.Reader, cfg *config.Config, cmd *cobra.Command) error {
	fmt.Println("\n=== Cloud Storage Configuration (S3-Compatible) ===")
	fmt.Println()
	fmt.Println("Supported providers: AWS S3, Cloudflare R2, Backblaze B2, Wasabi, MinIO")
	fmt.Println()

	cfg.StorageType = config.StorageTypeS3

	// Provider selection
	provider := promptSelect(reader, "Select provider", []string{"r2", "s3", "b2", "wasabi", "minio", "other"})
	if provider == "other" {
		provider = prompt(reader, "Enter provider name", "s3")
	}

	// Endpoint (required for most providers except AWS S3)
	var endpoint string
	switch provider {
	case "r2":
		endpoint = prompt(reader, "R2 Endpoint (https://<account>.r2.cloudflarestorage.com)", "")
	case "b2":
		endpoint = prompt(reader, "B2 S3 Endpoint (https://s3.<region>.backblazeb2.com)", "")
	case "minio":
		endpoint = prompt(reader, "MinIO Endpoint (http://localhost:9000)", "http://localhost:9000")
	case "wasabi":
		endpoint = prompt(reader, "Wasabi Endpoint (optional, press Enter for default)", "")
	case "s3":
		endpoint = prompt(reader, "S3 Endpoint (optional for AWS, press Enter to skip)", "")
	default:
		endpoint = prompt(reader, "Endpoint URL", "")
	}

	// Bucket
	bucket := prompt(reader, "Bucket name", "")
	if bucket == "" {
		return fmt.Errorf("bucket name is required")
	}

	// Region (optional for some providers)
	region := prompt(reader, "Region (optional)", "")

	// Access Key
	accessKey := promptPassword(reader, "Access Key ID")
	if accessKey == "" {
		return fmt.Errorf("access key is required")
	}

	// Secret Key
	secretKey := promptPassword(reader, "Secret Access Key")
	if secretKey == "" {
		return fmt.Errorf("secret key is required")
	}

	// Server ID
	serverID := prompt(reader, "Server ID (unique identifier for this server)", "")
	if serverID == "" {
		return fmt.Errorf("server ID is required")
	}

	// Optional prefix
	prefix := prompt(reader, "Key prefix (optional, e.g., 'backups')", "")

	// Build S3 config
	cfg.S3 = s3.Config{
		Provider:  provider,
		Endpoint:  endpoint,
		Bucket:    bucket,
		Region:    region,
		AccessKey: accessKey,
		SecretKey: secretKey,
		ServerID:  serverID,
		Prefix:    prefix,
		PathStyle: provider == "minio",
	}

	// Test connection
	fmt.Println("\nTesting cloud storage connection...")
	testCfg := &config.Config{
		StorageType: config.StorageTypeS3,
		S3:          cfg.S3,
		Paths:       cfg.Paths,
	}

	remoteStorage, err := remote.NewStorageWithContext(cmd.Context(), testCfg)
	if err != nil {
		fmt.Printf("Warning: Failed to connect to storage: %v\n", err)
		if !confirm(reader, "Continue anyway?") {
			return fmt.Errorf("initialization cancelled")
		}
	} else {
		fmt.Printf("Cloud storage connection successful! (%s)\n", remoteStorage.String())
	}

	return nil
}

func configureLocalStorage(reader *bufio.Reader, cfg *config.Config) error {
	fmt.Println("\n=== Local Storage Configuration ===")
	fmt.Println()
	fmt.Println("Backups will be stored locally on this machine.")
	fmt.Println("Note: Without cloud storage, backups will be lost if this server fails.")
	fmt.Println()

	cfg.StorageType = config.StorageTypeLocal

	// Custom path
	defaultPath := cfg.Paths.BackupsDir
	customPath := prompt(reader, fmt.Sprintf("Backup directory [%s]", defaultPath), "")
	if customPath == "" {
		customPath = defaultPath
	}

	// Expand home directory if needed
	if strings.HasPrefix(customPath, "~/") {
		home, _ := os.UserHomeDir()
		customPath = filepath.Join(home, customPath[2:])
	}

	cfg.Local.Path = customPath

	// Create directory if it doesn't exist
	if err := os.MkdirAll(customPath, 0750); err != nil {
		return fmt.Errorf("failed to create backup directory: %w", err)
	}

	// Retention days (prompt already returns default if empty, so just parse)
	retentionStr := prompt(reader, "Retention period - days to keep backups (0 = unlimited, default: 30)", "30")
	cfg.Local.RetentionDays, _ = strconv.Atoi(retentionStr)

	// Min free space (prompt already returns default if empty, so just parse)
	minSpaceStr := prompt(reader, "Minimum free space - warn if less than this many GB free (0 = no check, default: 10)", "10")
	cfg.Local.MinFreeSpaceGB, _ = strconv.Atoi(minSpaceStr)

	fmt.Println()
	fmt.Printf("Configuration:\n")
	fmt.Printf("  Path: %s\n", cfg.Local.Path)
	if cfg.Local.RetentionDays > 0 {
		fmt.Printf("  Retention: %d days\n", cfg.Local.RetentionDays)
	} else {
		fmt.Printf("  Retention: unlimited\n")
	}
	if cfg.Local.MinFreeSpaceGB > 0 {
		fmt.Printf("  Min free space: %d GB\n", cfg.Local.MinFreeSpaceGB)
	}

	return nil
}

func prompt(reader *bufio.Reader, label, defaultValue string) string {
	if defaultValue != "" {
		fmt.Printf("%s [%s]: ", label, defaultValue)
	} else {
		fmt.Printf("%s: ", label)
	}

	input, err := reader.ReadString('\n')
	if err != nil {
		return defaultValue
	}
	input = strings.TrimSpace(input)

	if input == "" {
		return defaultValue
	}

	return input
}

func promptSelect(reader *bufio.Reader, label string, options []string) string {
	fmt.Printf("%s:\n", label)
	for i, opt := range options {
		fmt.Printf("  [%d] %s\n", i+1, opt)
	}
	fmt.Print("Select [1]: ")

	input, err := reader.ReadString('\n')
	if err != nil {
		return options[0]
	}
	input = strings.TrimSpace(input)

	if input == "" {
		return options[0]
	}

	// Try to parse as number
	var idx int
	if _, err := fmt.Sscanf(input, "%d", &idx); err == nil && idx > 0 && idx <= len(options) {
		return options[idx-1]
	}

	// Otherwise use input directly
	return input
}

func promptPassword(reader *bufio.Reader, label string) string {
	fmt.Printf("%s: ", label)

	input, err := reader.ReadString('\n')
	if err != nil {
		return ""
	}
	return strings.TrimSpace(input)
}

func confirm(reader *bufio.Reader, message string) bool {
	fmt.Printf("%s (y/n): ", message)

	input, err := reader.ReadString('\n')
	if err != nil {
		return false
	}
	input = strings.TrimSpace(strings.ToLower(input))

	return input == "y" || input == "yes"
}
