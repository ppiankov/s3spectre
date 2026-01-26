package commands

import (
	"fmt"

	"github.com/spf13/cobra"
)

const version = "0.1.0"

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print version information",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("s3spectre version %s\n", version)
	},
}

// GetVersion returns the current version
func GetVersion() string {
	return version
}
