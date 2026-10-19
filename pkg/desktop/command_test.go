package desktop

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"runtime"
	"slices"
	"testing"
)

func TestDefaults(t *testing.T) {
	for _, tc := range []struct {
		os               string
		reveal, terminal []string
	}{
		{"darwin", []string{"open", "-R", "/dir/a b.txt"}, []string{"open", "-a", "Terminal", "/dir"}},
		{"windows", []string{"explorer.exe", "/select,/dir/a b.txt"}, []string{"powershell.exe", "-NoExit"}},
		{"linux", []string{"xdg-open", "/dir"}, []string{"x-terminal-emulator"}},
	} {
		reveal, terminal := Defaults(tc.os)
		if tc.os == "linux" {
			if err := json.Unmarshal([]byte(terminal), &tc.terminal); err != nil {
				t.Fatal(err)
			}
		}
		for i, template := range []string{reveal, terminal} {
			cmd, err := Command(template, "/dir/a b.txt", "/dir")
			if err != nil {
				t.Fatal(err)
			}
			want := tc.reveal
			if i == 1 {
				want = tc.terminal
			}
			if !slices.Equal(cmd.Args, want) || cmd.Dir != "/dir" {
				t.Errorf("%s: args=%q dir=%q", tc.os, cmd.Args, cmd.Dir)
			}
		}
	}
}

func TestLinuxTerminalFallback(t *testing.T) {
	for _, available := range []string{"x-terminal-emulator", "gnome-terminal", "konsole", "xterm", ""} {
		got := linuxTerminal(func(name string) (string, error) {
			if name == available {
				return name, nil
			}
			return "", fmt.Errorf("not installed")
		})
		want := available
		if want == "" {
			want = "x-terminal-emulator"
		}
		if got != fmt.Sprintf(`[%q]`, want) {
			t.Fatalf("available %q: %s", available, got)
		}
	}
}

func TestPrepareTerminal(t *testing.T) {
	cmd := exec.Command("terminal")
	PrepareTerminal(cmd)
	if (cmd.SysProcAttr != nil) != (runtime.GOOS == "windows") {
		t.Fatalf("unexpected console attributes: %+v", cmd.SysProcAttr)
	}
}

func TestCommandArguments(t *testing.T) {
	path := `/folder with spaces/";$(touch nope) 不.txt`
	cmd, err := Command(`["/app with spaces/terminal","--title=two words","{path}","{dir}"]`, path, "/folder with spaces")
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(cmd.Args, []string{"/app with spaces/terminal", "--title=two words", path, "/folder with spaces"}) {
		t.Fatalf("arguments changed: %q", cmd.Args)
	}
	for _, invalid := range []string{"", "null", "[]", `[""]`, `[" "]`, `"open"`, `[1]`, `["open",`, `["open","\u0000"]`} {
		if _, err := Command(invalid, path, "/dir"); err == nil {
			t.Errorf("accepted invalid command %q", invalid)
		}
	}
}
