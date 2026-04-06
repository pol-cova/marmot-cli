package discovery

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
)

// DockerDiscoverer implements database discovery using Docker
type DockerDiscoverer struct {
	cli *client.Client
}

// NewDockerDiscoverer creates a new Docker discoverer
func NewDockerDiscoverer() (*DockerDiscoverer, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, fmt.Errorf("failed to create Docker client: %w", err)
	}

	return &DockerDiscoverer{cli: cli}, nil
}

// Discover finds all databases in Docker containers
func (d *DockerDiscoverer) Discover(ctx context.Context) ([]DatabaseInfo, error) {
	containers, err := d.cli.ContainerList(ctx, types.ContainerListOptions{
		All: true,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list containers: %w", err)
	}

	var databases []DatabaseInfo

	for _, container := range containers {
		info, err := d.inspectContainer(ctx, container.ID, container.Names)
		if err != nil {
			continue
		}

		if info != nil {
			databases = append(databases, *info)
		}
	}

	return databases, nil
}

func (d *DockerDiscoverer) inspectContainer(ctx context.Context, containerID string, names []string) (*DatabaseInfo, error) {
	containerName := ""
	if len(names) > 0 {
		containerName = strings.TrimPrefix(names[0], "/")
	}

	inspect, err := d.cli.ContainerInspect(ctx, containerID)
	if err != nil {
		return nil, err
	}

	image := inspect.Config.Image
	env := make(map[string]string)
	for _, e := range inspect.Config.Env {
		parts := strings.SplitN(e, "=", 2)
		if len(parts) == 2 {
			env[parts[0]] = parts[1]
		}
	}

	var dbInfo *DatabaseInfo

	// Detect MySQL
	if strings.Contains(strings.ToLower(image), "mysql") {
		dbInfo = d.detectMySQL(containerID, containerName, env, inspect.NetworkSettings)
	}

	// Detect PostgreSQL
	if strings.Contains(strings.ToLower(image), "postgres") {
		dbInfo = d.detectPostgreSQL(containerID, containerName, env, inspect.NetworkSettings)
	}

	// Detect MongoDB
	if strings.Contains(strings.ToLower(image), "mongo") {
		dbInfo = d.detectMongoDB(containerID, containerName, env, inspect.NetworkSettings)
	}

	return dbInfo, nil
}

func (d *DockerDiscoverer) detectMongoDB(containerID, containerName string, env map[string]string, networkSettings *types.NetworkSettings) *DatabaseInfo {
	port := getPort(networkSettings, 27017, "27017")

	database := env["MONGO_INITDB_DATABASE"]
	if database == "" {
		database = env["MONGODB_DATABASE"]
	}
	if database == "" {
		database = "admin"
	}

	user := env["MONGO_INITDB_ROOT_USERNAME"]
	if user == "" {
		user = env["MONGODB_USERNAME"]
	}
	if user == "" {
		user = env["MONGO_USERNAME"]
	}
	if user == "" {
		user = "root"
	}

	password := env["MONGO_INITDB_ROOT_PASSWORD"]
	if password == "" {
		password = env["MONGODB_PASSWORD"]
	}
	if password == "" {
		password = env["MONGO_PASSWORD"]
	}

	return &DatabaseInfo{
		ContainerID:   containerID,
		ContainerName: containerName,
		Type:          DatabaseTypeMongoDB,
		Version:       env["MONGO_VERSION"],
		Host:          "localhost",
		Port:          port,
		Database:      database,
		User:          user,
		Password:      password,
		Environment:   env,
	}
}

func (d *DockerDiscoverer) detectMySQL(containerID, containerName string, env map[string]string, networkSettings *types.NetworkSettings) *DatabaseInfo {
	port := getPort(networkSettings, 3306, "3306")

	database := env["MYSQL_DATABASE"]
	if database == "" {
		database = "mysql"
	}

	user := env["MYSQL_USER"]
	if user == "" {
		user = "root"
	}

	password := env["MYSQL_ROOT_PASSWORD"]
	if password == "" {
		password = env["MYSQL_PASSWORD"]
	}

	return &DatabaseInfo{
		ContainerID:   containerID,
		ContainerName: containerName,
		Type:          DatabaseTypeMySQL,
		Version:       env["MYSQL_VERSION"],
		Host:          "localhost",
		Port:          port,
		Database:      database,
		User:          user,
		Password:      password,
		Environment:   env,
	}
}

func (d *DockerDiscoverer) detectPostgreSQL(containerID, containerName string, env map[string]string, networkSettings *types.NetworkSettings) *DatabaseInfo {
	port := getPort(networkSettings, 5432, "5432")

	database := env["POSTGRES_DB"]
	if database == "" {
		database = env["POSTGRES_USER"]
	}
	if database == "" {
		database = "postgres"
	}

	user := env["POSTGRES_USER"]
	if user == "" {
		user = "postgres"
	}

	password := env["POSTGRES_PASSWORD"]

	return &DatabaseInfo{
		ContainerID:   containerID,
		ContainerName: containerName,
		Type:          DatabaseTypePostgreSQL,
		Version:       env["POSTGRES_VERSION"],
		Host:          "localhost",
		Port:          port,
		Database:      database,
		User:          user,
		Password:      password,
		Environment:   env,
	}
}

func getPort(networkSettings *types.NetworkSettings, defaultPort int, portStr string) int {
	if networkSettings != nil && len(networkSettings.Ports) > 0 {
		for p, bindings := range networkSettings.Ports {
			ps := strings.Split(string(p), "/")[0]
			if ps == portStr && len(bindings) > 0 {
				if port, err := strconv.Atoi(ps); err == nil {
					return port
				}
			}
		}
	}
	return defaultPort
}
