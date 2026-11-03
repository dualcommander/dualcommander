package desktop

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

func clipboardTool(write bool) []string {
	if os.Getenv("WAYLAND_DISPLAY") != "" {
		name := "wl-paste"
		args := []string{"--no-newline", "--type", "text/uri-list"}
		if write {
			name, args = "wl-copy", []string{"--type", "text/uri-list"}
		}
		if _, err := exec.LookPath(name); err == nil {
			return append([]string{name}, args...)
		}
	}
	if os.Getenv("DISPLAY") != "" {
		if _, err := exec.LookPath("xclip"); err == nil {
			mode := "-out"
			if write {
				mode = "-in"
			}
			return []string{"xclip", "-selection", "clipboard", "-target", "text/uri-list", mode}
		}
	}
	return nil
}

// CopyFiles publishes URI references; the helper retains clipboard ownership.
func CopyFiles(paths []string) error {
	args := clipboardTool(true)
	if len(args) == 0 || len(paths) == 0 {
		return nil
	}
	var lines []string
	for _, path := range paths {
		if !filepath.IsAbs(path) || strings.ContainsRune(path, 0) {
			return fmt.Errorf("invalid clipboard file path: %q", path)
		}
		lines = append(lines, (&url.URL{Scheme: "file", Path: path}).String())
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, args[0], args[1:]...)
	cmd.Stdin = strings.NewReader(strings.Join(lines, "\r\n") + "\r\n")
	return cmd.Run()
}

func ReadFiles(ctx context.Context) ([]string, error) {
	args := clipboardTool(false)
	if len(args) == 0 {
		return nil, nil
	}
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, args[0], args[1:]...)
	data, err := cmd.Output()
	if _, noURIs := err.(*exec.ExitError); noURIs && ctx.Err() == nil {
		for i, arg := range args {
			if arg == "text/uri-list" {
				args[i] = "x-special/gnome-copied-files"
			}
		}
		data, err = exec.CommandContext(ctx, args[0], args[1:]...).Output()
	}
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		if _, ok := err.(*exec.ExitError); ok {
			return nil, nil // No file-URI target, including an empty/text clipboard.
		}
		return nil, err
	}
	return parseFileURIs(string(data))
}

func parseFileURIs(data string) ([]string, error) {
	if strings.HasPrefix(data, "copy\n") || strings.HasPrefix(data, "cut\n") {
		_, data, _ = strings.Cut(data, "\n")
	}
	var paths []string
	for _, line := range strings.Split(data, "\n") {
		line = strings.TrimSuffix(line, "\r")
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		u, err := url.Parse(line)
		if err != nil || u.Scheme != "file" || (u.Host != "" && u.Host != "localhost") || u.RawQuery != "" || u.Fragment != "" || !filepath.IsAbs(u.Path) || strings.ContainsRune(u.Path, 0) {
			return nil, fmt.Errorf("invalid local file URI in clipboard: %q", line)
		}
		paths = append(paths, u.Path)
	}
	return paths, nil
}
