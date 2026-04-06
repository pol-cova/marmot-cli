package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/pol-cova/marmot-cli/internal/s3"
	"github.com/spf13/viper"
)

// StorageType defines the type of storage backend
type StorageType string

const (
	StorageTypeLocal StorageType = "local"
	StorageTypeS3    StorageType = "s3"
)

// LocalStorageConfig holds configuration for local-only storage
type LocalStorageConfig struct {
	Path           string `mapstructure:"path" yaml:"path"`
	RetentionDays  int    `mapstructure:"retention_days" yaml:"retention_days"`
	MinFreeSpaceGB int    `mapstructure:"min_free_space_gb" yaml:"min_free_space_gb"`
}

// DatabaseConfig holds configuration for a single database
type DatabaseConfig struct {
	ID          string `mapstructure:"id" yaml:"id"`
	Type        string `mapstructure:"type" yaml:"type"` // mysql, postgres, mongo
	ContainerID string `mapstructure:"container_id" yaml:"container_id"`
	DSN         string `mapstructure:"dsn" yaml:"dsn"` // direct connection string (alternative to container_id)
	Name        string `mapstructure:"name" yaml:"name"`
	User        string `mapstructure:"user" yaml:"user"`
	Password    string `mapstructure:"password" yaml:"password"`
	Host        string `mapstructure:"host" yaml:"host"`
	Port        int    `mapstructure:"port" yaml:"port"`
	Schedule    string `mapstructure:"schedule" yaml:"schedule"` // Cron expression
	Enabled     bool   `mapstructure:"enabled" yaml:"enabled"`
}

// Config holds the complete application configuration
type Config struct {
	StorageType StorageType        `mapstructure:"storage_type" yaml:"storage_type"`
	Local       LocalStorageConfig `mapstructure:"local" yaml:"local"`
	S3          s3.Config          `mapstructure:"s3" yaml:"s3"`
	Databases   []DatabaseConfig   `mapstructure:"databases" yaml:"databases"`
	Paths       *Paths             `mapstructure:"-" yaml:"-"`
}

// IsLocal returns true if using local-only storage
func (c *Config) IsLocal() bool {
	return c.StorageType == StorageTypeLocal
}

// IsS3 returns true if using S3-compatible storage
func (c *Config) IsS3() bool {
	return c.StorageType == StorageTypeS3
}

// GetStoragePath returns the backup storage path (local or S3 bucket)
func (c *Config) GetStoragePath() string {
	if c.IsLocal() {
		if c.Local.Path != "" {
			return c.Local.Path
		}
		return c.Paths.BackupsDir
	}
	return c.S3.Bucket
}

// LoadConfig loads configuration from file and environment variables
func LoadConfig(configPath string) (*Config, error) {
	paths := GetDefaultPaths()
	v := viper.New()

	v.SetConfigType("yaml")

	// Set default config file path if not provided
	if configPath == "" {
		configPath = paths.ConfigFile
	}

	// Set config file
	v.SetConfigFile(configPath)

	// Set environment variable prefix
	v.SetEnvPrefix("MARMOT")
	v.AutomaticEnv()

	// Bind environment variables
	v.BindEnv("storage_type", "MARMOT_STORAGE_TYPE")

	// Local storage env vars
	v.BindEnv("local.path", "MARMOT_LOCAL_PATH")
	v.BindEnv("local.retention_days", "MARMOT_LOCAL_RETENTION_DAYS")
	v.BindEnv("local.min_free_space_gb", "MARMOT_LOCAL_MIN_FREE_SPACE_GB")

	// S3 env vars
	v.BindEnv("s3.provider", "MARMOT_S3_PROVIDER")
	v.BindEnv("s3.endpoint", "MARMOT_S3_ENDPOINT")
	v.BindEnv("s3.bucket", "MARMOT_S3_BUCKET")
	v.BindEnv("s3.region", "MARMOT_S3_REGION")
	v.BindEnv("s3.access_key", "MARMOT_S3_ACCESS_KEY")
	v.BindEnv("s3.secret_key", "MARMOT_S3_SECRET_KEY")
	v.BindEnv("s3.path_style", "MARMOT_S3_PATH_STYLE")
	v.BindEnv("s3.prefix", "MARMOT_S3_PREFIX")
	v.BindEnv("s3.server_id", "MARMOT_S3_SERVER_ID")

	// Read config file (optional - may not exist on first run)
	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("failed to read config file: %w", err)
		}
	}

	var config Config
	if err := v.Unmarshal(&config); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	config.Paths = paths

	return &config, nil
}

// SaveConfig saves configuration to file
func SaveConfig(config *Config, configPath string) error {
	if configPath == "" {
		configPath = config.Paths.ConfigFile
	}

	// Ensure config directory exists
	configDir := filepath.Dir(configPath)
	if err := os.MkdirAll(configDir, 0700); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	// Create new viper instance for writing
	v := viper.New()
	v.SetConfigType("yaml")
	v.SetConfigFile(configPath)

	// Set values
	v.Set("storage_type", config.StorageType)
	v.Set("local", config.Local)
	v.Set("s3", config.S3)
	v.Set("databases", config.Databases)

	// Write config file with secure permissions
	if err := v.WriteConfigAs(configPath); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	// Set secure file permissions (0600)
	if err := os.Chmod(configPath, 0600); err != nil {
		return fmt.Errorf("failed to set config file permissions: %w", err)
	}

	return nil
}

// Validate checks if the configuration is valid
func (c *Config) Validate() error {
	// Validate storage type
	switch c.StorageType {
	case StorageTypeLocal:
		if err := c.validateLocalConfig(); err != nil {
			return err
		}
	case StorageTypeS3:
		if err := c.validateS3Config(); err != nil {
			return err
		}
	default:
		return fmt.Errorf("storage_type must be 'local' or 's3'")
	}

	// Validate databases
	for i, db := range c.Databases {
		if db.ID == "" {
			return fmt.Errorf("database[%d]: ID is required", i)
		}

		if db.Type != "mysql" && db.Type != "postgres" && db.Type != "mongo" {
			return fmt.Errorf("database[%d]: type must be 'mysql', 'postgres', or 'mongo'", i)
		}

		if db.ContainerID == "" && db.DSN == "" {
			return fmt.Errorf("database[%d]: either container_id or dsn is required", i)
		}
	}

	return nil
}

func (c *Config) validateLocalConfig() error {
	return nil
}

func (c *Config) validateS3Config() error {
	if err := c.S3.Validate(); err != nil {
		return fmt.Errorf("s3 configuration invalid: %w", err)
	}
	return nil
}

// GetServerID returns the server ID (for S3) or machine identifier (for local)
func (c *Config) GetServerID() string {
	if c.IsS3() {
		return c.S3.ServerID
	}
	// For local storage, use hostname or default
	hostname, _ := os.Hostname()
	if hostname == "" {
		return "local"
	}
	return hostname
}

// GetDatabaseByID returns a database configuration by ID
func (c *Config) GetDatabaseByID(id string) *DatabaseConfig {
	for i := range c.Databases {
		if c.Databases[i].ID == id {
			return &c.Databases[i]
		}
	}
	return nil
}

// ShouldCleanup returns true if cleanup should be performed based on retention/free space
func (c *Config) ShouldCleanup() bool {
	if !c.IsLocal() {
		return false
	}
	return c.Local.RetentionDays > 0 || c.Local.MinFreeSpaceGB > 0
}
