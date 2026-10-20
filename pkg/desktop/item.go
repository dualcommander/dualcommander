package desktop

import (
	_ "embed"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

//go:embed properties.applescript
var propertiesScript string

//go:embed properties.ps1
var windowsPropertiesScript string

// ItemCommand delegates application choice and properties to native OS handlers.
// Paths remain data, never executable script text.
func ItemCommand(goos, action, path string) (*exec.Cmd, error) {
	if action != "open-with" && action != "properties" {
		return nil, fmt.Errorf("unsupported item action: %s", action)
	}
	var cmd *exec.Cmd
	switch goos {
	case "darwin":
		if action == "open-with" {
			return nil, fmt.Errorf("Open With requires the native application panel")
		}
		cmd = exec.Command("osascript", "-e", propertiesScript, "--", path)
	case "windows":
		if action == "open-with" {
			cmd = exec.Command("rundll32.exe", "shell32.dll,OpenAs_RunDLL", path)
		} else {
			cmd = exec.Command("powershell.exe", "-NoProfile", "-NonInteractive", "-Command", windowsPropertiesScript)
			cmd.Env = append(os.Environ(), "DUALCOMMANDER_ITEM="+path)
		}
	default:
		return nil, fmt.Errorf("native %s unavailable on %s; use the file manager", action, goos)
	}
	cmd.Dir = filepath.Dir(path)
	return cmd, nil
}
