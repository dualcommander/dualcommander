package main

import (
	"bytes"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Debug logging is opt-in: DUALCOMMANDER_DEBUG=1 writes Debug records to a file beside the executable.
func TestDebugLogOptIn(t *testing.T) {
	prev := slog.Default()
	t.Cleanup(func() { slog.SetDefault(prev) })
	dir := t.TempDir()
	exe := filepath.Join(dir, "dualcommander")

	var stderr bytes.Buffer
	closeLog, err := setupLogging(func(string) string { return "" }, exe, &stderr)
	if err != nil {
		t.Fatal(err)
	}
	slog.Debug("hidden without opt-in")
	closeLog()
	if _, err := os.Stat(filepath.Join(dir, debugLogName)); !os.IsNotExist(err) {
		t.Fatalf("debug file created without opt-in: %v", err)
	}
	if strings.Contains(stderr.String(), "hidden without opt-in") {
		t.Fatal("debug record printed without opt-in")
	}

	env := func(key string) string {
		if key == debugEnv {
			return "1"
		}
		return ""
	}
	closeLog, err = setupLogging(env, exe, &stderr)
	if err != nil {
		t.Fatal(err)
	}
	slog.Debug("visible with opt-in", "dir", dir)
	slog.Info("info too")
	closeLog()

	data, err := os.ReadFile(filepath.Join(dir, debugLogName))
	if err != nil {
		t.Fatalf("debug file missing beside executable: %v", err)
	}
	for _, want := range []string{"visible with opt-in", "info too"} {
		if !strings.Contains(string(data), want) {
			t.Errorf("debug file missing %q:\n%s", want, data)
		}
	}
	if !strings.Contains(stderr.String(), "visible with opt-in") {
		t.Error("debug record not mirrored to stderr")
	}
}
