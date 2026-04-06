package main

import (
	"fmt"
	"os"
	"runtime"

	"github.com/pol-cova/marmot-cli/cmd/marmot/cmd"
)

// These variables are set at build time via ldflags
var (
	version   = "dev"     // Set by: -X main.version=$(VERSION)
	commit    = "unknown" // Set by: -X main.commit=$(COMMIT)
	buildTime = "unknown" // Set by: -X main.buildTime=$(BUILD_TIME)
)

func main() {
	versionInfo := &cmd.VersionInfo{
		Version:   version,
		Commit:    commit,
		BuildTime: buildTime,
		GoVersion: runtime.Version(),
		OS:        runtime.GOOS,
		Arch:      runtime.GOARCH,
	}

	if err := cmd.Execute(versionInfo); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
