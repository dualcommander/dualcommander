package desktop

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// Windows Forms uses native CF_HDROP file references, compatible with Explorer.
const fileClipboardScript = `
$ErrorActionPreference = 'Stop'
[Console]::InputEncoding = New-Object System.Text.UTF8Encoding($false)
[Console]::OutputEncoding = New-Object System.Text.UTF8Encoding($false)
Add-Type -AssemblyName System.Windows.Forms
if ($env:DUALCOMMANDER_CLIPBOARD_WRITE -eq '1') {
    $paths = ConvertFrom-Json ([Console]::In.ReadToEnd())
    $files = New-Object System.Collections.Specialized.StringCollection
    foreach ($path in $paths) { [void]$files.Add($path) }
    [System.Windows.Forms.Clipboard]::SetFileDropList($files)
} elseif ([System.Windows.Forms.Clipboard]::ContainsFileDropList()) {
    $files = @([System.Windows.Forms.Clipboard]::GetFileDropList())
    ConvertTo-Json -InputObject $files -Compress
} else { '[]' }
`

func CopyFiles(paths []string) error {
	if len(paths) == 0 {
		return nil
	}
	for _, path := range paths {
		if !filepath.IsAbs(path) || strings.ContainsRune(path, 0) {
			return fmt.Errorf("invalid clipboard file path: %q", path)
		}
	}
	data, err := json.Marshal(paths)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cmd := clipboardCommand(ctx)
	cmd.Env = append(cmd.Environ(), "DUALCOMMANDER_CLIPBOARD_WRITE=1")
	cmd.Stdin = strings.NewReader(string(data))
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("write file clipboard: %w: %s", err, output)
	}
	return nil
}

func clipboardCommand(ctx context.Context) *exec.Cmd {
	return exec.CommandContext(ctx, "powershell.exe", "-NoProfile", "-NonInteractive", "-STA", "-Command", fileClipboardScript)
}

func ReadFiles(ctx context.Context) ([]string, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	cmd := clipboardCommand(ctx)
	cmd.Env = append(cmd.Environ(), "DUALCOMMANDER_CLIPBOARD_WRITE=0")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("read file clipboard: %w", err)
	}
	var paths []string
	if err := json.Unmarshal(output, &paths); err != nil {
		return nil, err
	}
	for _, path := range paths {
		if !filepath.IsAbs(path) || strings.ContainsRune(path, 0) {
			return nil, fmt.Errorf("invalid clipboard file path: %q", path)
		}
	}
	return paths, nil
}
