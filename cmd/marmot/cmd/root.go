package cmd

import (
	"fmt"
	"runtime"

	"github.com/pol-cova/marmot-cli/internal/config"

	"github.com/spf13/cobra"
)

var (
	configPath string
	verbose    bool
	rootCmd    *cobra.Command
)

// VersionInfo holds build-time version information
type VersionInfo struct {
	Version   string
	Commit    string
	BuildTime string
	GoVersion string
	OS        string
	Arch      string
}

// Execute runs the root command
func Execute(v *VersionInfo) error {
	versionStr := v.Version
	if v.Commit != "" && v.Commit != "unknown" {
		versionStr = fmt.Sprintf("%s (%s)", v.Version, v.Commit)
	}

	rootCmd = &cobra.Command{
		Use:   "marmot",
		Short: "Marmot - Database backup tool",
		Long: getBanner() + `
Marmot automatically backs up your databases to S3-compatible storage.
Supports MySQL, PostgreSQL, and MongoDB with encryption and compression.`,
		Version: versionStr,
		Run: func(cmd *cobra.Command, args []string) {
			cmd.Help()
		},
	}

	// Add version flag handler to show full version info
	rootCmd.SetVersionTemplate(`{{with .Name}}{{printf "%s " .}}{{end}}{{printf "%s" .Version}}
`)

	// Add a custom --version-full flag
	rootCmd.PersistentFlags().StringVar(&configPath, "config", "", "config file (default is platform-specific)")
	rootCmd.PersistentFlags().BoolVar(&verbose, "verbose", false, "verbose output")

	// Override default version command to show full info
	rootCmd.InitDefaultVersionFlag()

	// Store version info for potential use in other commands
	rootCmd.Annotations = map[string]string{
		"version":   v.Version,
		"commit":    v.Commit,
		"buildTime": v.BuildTime,
		"goVersion": v.GoVersion,
		"os":        v.OS,
		"arch":      v.Arch,
	}

	rootCmd.AddCommand(newInitCmd())
	rootCmd.AddCommand(newBackupCmd())
	rootCmd.AddCommand(newStatusCmd())
	rootCmd.AddCommand(newRestoreCmd())
	rootCmd.AddCommand(newStartCmd())
	rootCmd.AddCommand(newQueueCmd())
	rootCmd.AddCommand(newCleanupCmd())
	rootCmd.AddCommand(newDecryptCmd())
	rootCmd.AddCommand(newKeyCmd())
	rootCmd.AddCommand(newDbCmd())
	rootCmd.AddCommand(newServiceCmd())
	rootCmd.AddCommand(newVerifyCmd())
	rootCmd.AddCommand(newDaemonCmd())

	return rootCmd.Execute()
}

// GetVersionInfo retrieves version info from command annotations
func GetVersionInfo() *VersionInfo {
	if rootCmd == nil {
		return &VersionInfo{
			Version:   "dev",
			GoVersion: runtime.Version(),
			OS:        runtime.GOOS,
			Arch:      runtime.GOARCH,
		}
	}

	return &VersionInfo{
		Version:   rootCmd.Annotations["version"],
		Commit:    rootCmd.Annotations["commit"],
		BuildTime: rootCmd.Annotations["buildTime"],
		GoVersion: rootCmd.Annotations["goVersion"],
		OS:        rootCmd.Annotations["os"],
		Arch:      rootCmd.Annotations["arch"],
	}
}

func getConfigPath() string {
	if configPath != "" {
		return configPath
	}

	// Use default path from config package
	paths := config.GetDefaultPaths()
	return paths.ConfigFile
}

// GetDefaultPaths is a helper that wraps config.GetDefaultPaths
func GetDefaultPaths() *config.Paths {
	return config.GetDefaultPaths()
}

func getBanner() string {
	return `
███╗   ███╗ █████╗ ██████╗ ███╗   ███╗ ██████╗ ████████╗
████╗ ████║██╔══██╗██╔══██╗████╗ █████║██╔═══██╗╚══██╔══╝
██╔████╔██║███████║██████╔╝██╔████╔██║██║   ██║   ██║   
██║╚██╔╝██║██╔══██║██╔══██╗██║╚██╔╝██║██║   ██║   ██║   
██║ ╚═╝ ██║██║  ██║██║  ██║██║ ╚═╝ ██║╚██████╔╝   ██║   
╚═╝     ╚═╝╚═╝  ╚═╝╚═╝  ╚═╝╚═╝     ╚═╝ ╚═════╝    ╚═╝   
                                                          
  S3-Compatible Database Backups
  github.com/pol-cova/marmot-cli
`
}
