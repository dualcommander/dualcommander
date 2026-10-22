// Package desktop builds shell-free commands for desktop applications.
package desktop

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"runtime"
	"strings"
)

// Defaults returns reveal and terminal commands as JSON argument lists.
func Defaults(goos string) (reveal, terminal string) {
	switch goos {
	case "darwin":
		return `["open","-R","{path}"]`, `["open","-a","Terminal","{dir}"]`
	case "windows":
		return `["explorer.exe","/select,{path}"]`, `["powershell.exe","-NoExit"]`
	default:
		// ponytail: xdg-open opens parent folder; use FileManager1 for file selection later.
		return `["xdg-open","{dir}"]`, linuxTerminal(exec.LookPath)
	}
}

// FolderCommand opens dir itself, rather than revealing a selected child.
func FolderCommand(goos, dir string) (*exec.Cmd, error) {
	command := `["xdg-open","{dir}"]`
	switch goos {
	case "darwin":
		command = `["open","{dir}"]`
	case "windows":
		command = `["explorer.exe","{dir}"]`
	}
	return Command(command, dir, dir)
}

func linuxTerminal(lookPath func(string) (string, error)) string {
	for _, name := range []string{"x-terminal-emulator", "kgx", "gnome-terminal", "konsole", "xfce4-terminal", "mate-terminal", "kitty", "alacritty", "xterm"} {
		if _, err := lookPath(name); err == nil {
			args, _ := json.Marshal([]string{name})
			return string(args)
		}
	}
	return `["x-terminal-emulator"]` // missing terminal reports normal launch error
}

// PrepareTerminal gives Windows GUI launches their own console, including custom commands.
func PrepareTerminal(cmd *exec.Cmd) {
	if runtime.GOOS == "windows" {
		prepareTerminal(cmd)
	}
}

// Command substitutes paths after JSON parsing, keeping each argument intact.
// The working directory lets terminals without a directory flag open in dir.
func Command(template, path, dir string) (*exec.Cmd, error) {
	return command(template, dir, strings.NewReplacer("{path}", path, "{dir}", dir))
}

// ComparisonCommand keeps pane order and substitutes both paths without a shell.
func ComparisonCommand(template, left, right, dir string) (*exec.Cmd, error) {
	return command(template, dir, strings.NewReplacer("{left}", left, "{right}", right), "{left}", "{right}")
}

func command(template, dir string, replace *strings.Replacer, required ...string) (*exec.Cmd, error) {
	var args []string
	if err := json.Unmarshal([]byte(template), &args); err != nil {
		return nil, fmt.Errorf("command must be a JSON list of strings: %w", err)
	}
	if len(args) == 0 || strings.TrimSpace(args[0]) == "" {
		return nil, fmt.Errorf("command needs an executable")
	}
	for _, placeholder := range required {
		if !strings.Contains(strings.Join(args, "\x00"), placeholder) {
			return nil, fmt.Errorf("comparison command needs %s", placeholder)
		}
	}
	for i := range args {
		args[i] = replace.Replace(args[i])
		if strings.ContainsRune(args[i], 0) {
			return nil, fmt.Errorf("command cannot contain NUL characters")
		}
	}
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Dir = dir
	return cmd, nil
}
