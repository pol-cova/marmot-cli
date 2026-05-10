package discovery

import (
	"bytes"
	"context"
	"fmt"
	"net"
	"os/exec"
	"regexp"
	"runtime"
	"strings"
	"time"
)

// LocalDiscoverer implements database discovery on the local host.
// It uses service managers (brew/systemd) and port probing.
type LocalDiscoverer struct {
	timeout time.Duration
}

// NewLocalDiscoverer creates a new local discoverer with default timeouts.
func NewLocalDiscoverer() *LocalDiscoverer {
	return &LocalDiscoverer{timeout: 400 * time.Millisecond}
}

// Discover finds local databases running on the host (non-Docker).
func (d *LocalDiscoverer) Discover(ctx context.Context) ([]DatabaseInfo, error) {
	var databases []DatabaseInfo

	services := d.detectServices(ctx)
	openPorts := d.detectOpenPorts()

	if services.hasPostgres || openPorts.hasPostgres {
		databases = append(databases, DatabaseInfo{
			Type:     DatabaseTypePostgreSQL,
			Host:     "localhost",
			Port:     5432,
			Database: "postgres",
			User:     "postgres",
			Password: "",
		})
	}

	if services.hasMySQL || openPorts.hasMySQL {
		databases = append(databases, DatabaseInfo{
			Type:     DatabaseTypeMySQL,
			Host:     "localhost",
			Port:     3306,
			Database: "mysql",
			User:     "root",
			Password: "",
		})
	}

	if services.hasMongo || openPorts.hasMongo {
		databases = append(databases, DatabaseInfo{
			Type:     DatabaseTypeMongoDB,
			Host:     "localhost",
			Port:     27017,
			Database: "admin",
			User:     "root",
			Password: "",
		})
	}

	return databases, nil
}

type servicePresence struct {
	hasPostgres bool
	hasMySQL    bool
	hasMongo    bool
}

func (d *LocalDiscoverer) detectServices(ctx context.Context) servicePresence {
	switch runtime.GOOS {
	case "darwin":
		return d.detectBrewServices(ctx)
	case "linux":
		return d.detectSystemdServices(ctx)
	default:
		return servicePresence{}
	}
}

func (d *LocalDiscoverer) detectBrewServices(ctx context.Context) servicePresence {
	cmd := exec.CommandContext(ctx, "brew", "services", "list")
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		return servicePresence{}
	}

	text := strings.ToLower(out.String())
	return servicePresence{
		hasPostgres: hasAnyService(text, []string{"postgresql", "postgres", "postgresql@"}),
		hasMySQL:    hasAnyService(text, []string{"mysql", "mysql@", "mariadb"}),
		hasMongo:    hasAnyService(text, []string{"mongodb-community", "mongodb"}),
	}
}

func (d *LocalDiscoverer) detectSystemdServices(ctx context.Context) servicePresence {
	cmd := exec.CommandContext(ctx, "systemctl", "--type=service", "--state=running", "--no-pager", "--no-legend")
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		return servicePresence{}
	}

	text := strings.ToLower(out.String())
	return servicePresence{
		hasPostgres: hasAnyService(text, []string{"postgresql", "postgres"}),
		hasMySQL:    hasAnyService(text, []string{"mysql", "mariadb"}),
		hasMongo:    hasAnyService(text, []string{"mongod", "mongodb"}),
	}
}

func hasAnyService(text string, names []string) bool {
	for _, name := range names {
		re := regexp.MustCompile(`(?m)^` + regexp.QuoteMeta(name) + `\b`)
		if re.MatchString(text) {
			return true
		}
		if strings.Contains(text, name) {
			return true
		}
	}
	return false
}

type portPresence struct {
	hasPostgres bool
	hasMySQL    bool
	hasMongo    bool
}

func (d *LocalDiscoverer) detectOpenPorts() portPresence {
	return portPresence{
		hasPostgres: canConnect("127.0.0.1", 5432, d.timeout),
		hasMySQL:    canConnect("127.0.0.1", 3306, d.timeout),
		hasMongo:    canConnect("127.0.0.1", 27017, d.timeout),
	}
}

func canConnect(host string, port int, timeout time.Duration) bool {
	addr := fmt.Sprintf("%s:%d", host, port)
	conn, err := net.DialTimeout("tcp", addr, timeout)
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}
