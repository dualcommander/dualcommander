package main

import (
	"io"
	"log/slog"
	"os"
	"path/filepath"
)

const (
	debugEnv     = "DUALCOMMANDER_DEBUG"
	debugLogName = "dualcommander-debug.log"
)

// setupLogging sets the default logger. Without DUALCOMMANDER_DEBUG=1 it logs Info to stderr only.
// With it, Debug records also go to dualcommander-debug.log beside the executable. The file is
// truncated per start; graphics child processes (graphicsModeEnv set) append to the parent's file.
// On a file error the logger still works on stderr and the error is returned.
func setupLogging(getenv func(string) string, exe string, stderr io.Writer) (func(), error) {
	level, w, closeFile := slog.LevelInfo, stderr, func() {}
	var err error
	if getenv(debugEnv) == "1" {
		level = slog.LevelDebug
		var f *os.File
		f, err = openDebugLog(exe, getenv(graphicsModeEnv) == "")
		if err == nil {
			w = io.MultiWriter(stderr, f)
			closeFile = func() { f.Close() }
		}
	}
	slog.SetDefault(slog.New(slog.NewTextHandler(w, &slog.HandlerOptions{Level: level})).With("app", "DualCommander"))
	return closeFile, err
}

func openDebugLog(exe string, truncate bool) (*os.File, error) {
	flags := os.O_CREATE | os.O_WRONLY | os.O_APPEND
	if truncate {
		flags |= os.O_TRUNC
	}
	return os.OpenFile(filepath.Join(filepath.Dir(exe), debugLogName), flags, 0o600)
}
