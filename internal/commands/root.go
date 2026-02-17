package commands

import (
	"github.com/ppiankov/s3spectre/internal/logging"
	"github.com/spf13/cobra"
)

var verbose bool

var rootCmd = &cobra.Command{
	Use:   "s3spectre",
	Short: "S3Spectre - AWS S3 bucket usage auditor",
	Long: `S3Spectre scans code repositories for S3 bucket and prefix references,
validates them against your AWS S3 infrastructure, and identifies missing
buckets, unused buckets, stale prefixes, and lifecycle misconfigurations.

Part of the Spectre family of infrastructure cleanup tools.`,
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		logging.Init(verbose)
	},
}

func Execute() error {
	return rootCmd.Execute()
}

func init() {
	rootCmd.PersistentFlags().BoolVar(&verbose, "verbose", false, "Enable verbose logging")
	rootCmd.AddCommand(scanCmd)
	rootCmd.AddCommand(discoverCmd)
	rootCmd.AddCommand(versionCmd)
}
