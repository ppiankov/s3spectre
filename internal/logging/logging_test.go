package logging

import (
	"io"
	"log/slog"
	"os"
	"strings"
	"testing"
)

func captureStderr(t *testing.T, fn func()) string {
	t.Helper()

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("failed to create pipe: %v", err)
	}
	oldStderr := os.Stderr
	oldLogger := slog.Default()
	os.Stderr = w
	defer func() {
		os.Stderr = oldStderr
		slog.SetDefault(oldLogger)
	}()

	fn()

	_ = w.Close()
	out, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("failed to read stderr: %v", err)
	}
	return string(out)
}

func TestInitDefaultLevelWarn(t *testing.T) {
	output := captureStderr(t, func() {
		Init(false)
		slog.Info("info")
		slog.Warn("warn")
	})

	if strings.Contains(output, "msg=info") {
		t.Fatalf("expected info to be suppressed, got %q", output)
	}
	if !strings.Contains(output, "msg=warn") {
		t.Fatalf("expected warn to be logged, got %q", output)
	}
}

func TestInitVerboseLevelDebug(t *testing.T) {
	output := captureStderr(t, func() {
		Init(true)
		slog.Debug("debug")
		slog.Info("info")
	})

	if !strings.Contains(output, "msg=debug") {
		t.Fatalf("expected debug to be logged, got %q", output)
	}
	if !strings.Contains(output, "msg=info") {
		t.Fatalf("expected info to be logged, got %q", output)
	}
}
