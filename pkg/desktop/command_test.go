package desktop

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"runtime"
	"slices"
	"strings"
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

func TestFolderCommand(t *testing.T) {
	dir := `/folder with spaces/";$(touch nope) 世界`
	for _, tc := range []struct{ os, executable string }{
		{"darwin", "open"}, {"windows", "explorer.exe"}, {"linux", "xdg-open"},
	} {
		cmd, err := FolderCommand(tc.os, dir)
		if err != nil || !slices.Equal(cmd.Args, []string{tc.executable, dir}) || cmd.Dir != dir {
			t.Fatalf("%s: folder command=%v error=%v", tc.os, cmd, err)
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
	for _, invalid := range []string{"", "null", "[]", `[""]`, `[" "]`, `"open"`, `[1]`, `["open",`, `["open",null]`, `["open","\u0000"]`} {
		if _, err := Command(invalid, path, "/dir"); err == nil {
			t.Errorf("accepted invalid command %q", invalid)
		}
	}
}

func TestComparisonCommand(t *testing.T) {
	left, right := `/left space/";$(touch nope){right}`, `/right space/世界{left}`
	cmd, err := ComparisonCommand(`["/tool with spaces/diff","--left={left}","{right}"]`, left, right, "/left space")
	if err != nil || !slices.Equal(cmd.Args, []string{"/tool with spaces/diff", "--left=" + left, right}) || cmd.Dir != "/left space" {
		t.Fatalf("comparison arguments: %v %v", cmd, err)
	}
	for _, invalid := range []string{"", `["tool"]`, `["tool","{left}"]`, `["tool","{right}"]`, `["tool","{left}","{right}","\u0000"]`} {
		if _, err := ComparisonCommand(invalid, left, right, "/"); err == nil {
			t.Fatalf("accepted %s", invalid)
		}
	}
}

func TestCommandValidationLocation(t *testing.T) {
	for _, tc := range []struct{ template, location string }{
		{`["open",null]`, "/1"},
		{`["open",true]`, "/1"},
		{`["open",{}]`, "/1"},
		{`["open",[]]`, "/1"},
		{`["\u2003\t"]`, "/0"},
	} {
		_, err := Command(tc.template, "/file", "/dir")
		if err == nil || !strings.Contains(err.Error(), tc.location) {
			t.Errorf("%s: want error identifying %s, got %v", tc.template, tc.location, err)
		}
	}
}

func TestComparisonCommandEscapedPlaceholders(t *testing.T) {
	cmd, err := ComparisonCommand(`["diff","\u007bleft\u007d","\u007bright\u007d"]`, "/left file", "/right file", "/")
	if err != nil {
		t.Fatalf("valid JSON-escaped placeholders rejected: %v", err)
	}
	if !slices.Equal(cmd.Args, []string{"diff", "/left file", "/right file"}) {
		t.Fatalf("escaped placeholders not substituted: %q", cmd.Args)
	}
}
