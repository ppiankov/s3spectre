package main

import (
	"log/slog"
	"os"

	"github.com/ppiankov/s3spectre/internal/commands"
)

func main() {
	if err := commands.Execute(); err != nil {
		slog.Warn("Command failed", "error", err)
		os.Exit(1)
	}
}
