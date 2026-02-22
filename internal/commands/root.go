package commands

import (
	"log/slog"

	"github.com/ppiankov/s3spectre/internal/config"
	"github.com/ppiankov/s3spectre/internal/logging"
	"github.com/spf13/cobra"
)

var (
	verbose bool
	version string
	commit  string
	date    string
	cfg     config.Config
)

var rootCmd = &cobra.Command{
	Use:   "s3spectre",
	Short: "S3Spectre - AWS S3 bucket usage auditor",
	Long: `S3Spectre scans code repositories for S3 bucket and prefix references,
validates them against your AWS S3 infrastructure, and identifies missing
buckets, unused buckets, stale prefixes, and lifecycle misconfigurations.

Part of the Spectre family of infrastructure cleanup tools.`,
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		logging.Init(verbose)
		loaded, err := config.Load(".")
		if err != nil {
			slog.Warn("Failed to load config file", "error", err)
		} else {
			cfg = loaded
		}
	},
}

// Execute runs the root command with injected build info.
func Execute(v, c, d string) error {
	version = v
	commit = c
	date = d
	return rootCmd.Execute()
}

// GetVersion returns the current version.
func GetVersion() string {
	return version
}

func init() {
	rootCmd.PersistentFlags().BoolVar(&verbose, "verbose", false, "Enable verbose logging")
	rootCmd.AddCommand(scanCmd)
	rootCmd.AddCommand(discoverCmd)
	rootCmd.AddCommand(versionCmd)
}
