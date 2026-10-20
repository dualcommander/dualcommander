package desktop

import (
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestItemCommand(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, `space " ; $(touch nope) 世界.txt`)
	for _, goos := range []string{"darwin", "windows"} {
		for _, action := range []string{"open-with", "properties"} {
			cmd, err := ItemCommand(goos, action, path)
			if goos == "darwin" && action == "open-with" {
				if err == nil {
					t.Fatal("macOS chooser must use native panel, not subprocess")
				}
				continue
			}
			if err != nil || cmd.Dir != dir {
				t.Fatalf("%s %s: %v %v", goos, action, cmd, err)
			}
			if goos == "windows" && action == "properties" {
				if !slices.Contains(cmd.Env, "DUALCOMMANDER_ITEM="+path) || strings.Contains(cmd.Args[len(cmd.Args)-1], path) {
					t.Fatal("properties path must be data, not script")
				}
			} else if cmd.Args[len(cmd.Args)-1] != path {
				t.Fatalf("path argument changed: %q", cmd.Args)
			}
		}
	}
	for _, tc := range [][2]string{{"linux", "properties"}, {"darwin", "invalid"}} {
		if _, err := ItemCommand(tc[0], tc[1], path); err == nil {
			t.Fatal("unsupported action accepted")
		}
	}
}

func TestContextMenuWithoutWindow(t *testing.T) {
	if ChooseApplication(0) != "" {
		t.Fatal("missing native window must cancel application choice")
	}
	if ContextMenu(0, []ContextItem{{Title: "Open"}}, 0, 0) != -1 {
		t.Fatal("missing native window must cancel menu")
	}
}
