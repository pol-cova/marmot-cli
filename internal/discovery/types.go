package discovery

import "context"

// DatabaseType represents the type of database
type DatabaseType string

const (
	DatabaseTypeMySQL      DatabaseType = "mysql"
	DatabaseTypePostgreSQL DatabaseType = "postgres"
	DatabaseTypeMongoDB    DatabaseType = "mongo"
)

// DatabaseInfo holds information about a discovered database
type DatabaseInfo struct {
	ContainerID   string
	ContainerName string
	Type          DatabaseType
	Version       string
	Host          string
	Port          int
	Database      string
	User          string
	Password      string
	Environment   map[string]string
}

// Discoverer defines the interface for database discovery
type Discoverer interface {
	// Discover finds databases based on the implementation strategy
	Discover(ctx context.Context) ([]DatabaseInfo, error)
}
