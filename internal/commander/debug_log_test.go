package commander

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"github.com/dualcommander/dualcommander/pkg/fsops"
)

// Navigation and listing emit Debug records so the opt-in debug log shows what the pane did.
func TestKeyAndJobsLogDebugRecords(t *testing.T) {
	prev := slog.Default()
	t.Cleanup(func() { slog.SetDefault(prev) })
	root := fixture(t)
	u := newTestUI(t, test.NewTempApp(t), filepath.Join(root, "src"), filepath.Join(root, "dst"))
	var buf bytes.Buffer
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})))
	press(u, fyne.KeyDown)
	u.jobs.run("probe", nil, func(context.Context, fsops.Progress) error { return nil })
	waitJobs(t, u)
	out := buf.String()
	for _, want := range []string{"msg=key", "name=Down", "msg=\"job start\"", "msg=\"job done\""} {
		if !strings.Contains(out, want) {
			t.Errorf("debug log missing %q:\n%s", want, out)
		}
	}
}

func TestNavigationLogsDebugRecords(t *testing.T) {
	prev := slog.Default()
	t.Cleanup(func() { slog.SetDefault(prev) })
	root := fixture(t)
	u := newTestUI(t, test.NewTempApp(t), filepath.Join(root, "src"), filepath.Join(root, "src"))
	var buf bytes.Buffer
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})))
	u.left.navigate(filepath.Join(root, "dst"), "")
	out := buf.String()
	for _, want := range []string{"msg=navigate", `msg="listing folder"`, "entries="} {
		if !strings.Contains(out, want) {
			t.Errorf("debug log missing %q:\n%s", want, out)
		}
	}
}

// Debug output stays bounded: per-row and per-cell widget callbacks never log, whatever the folder size.
func TestNavigationDebugOutputIsBounded(t *testing.T) {
	prev := slog.Default()
	t.Cleanup(func() { slog.SetDefault(prev) })
	root := fixture(t)
	big := filepath.Join(root, "big")
	if err := os.MkdirAll(big, 0o755); err != nil {
		t.Fatal(err)
	}
	for i := range 300 {
		if err := os.WriteFile(filepath.Join(big, fmt.Sprintf("f%04d.txt", i)), nil, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	for _, brief := range []bool{false, true} {
		u := newTestUI(t, test.NewTempApp(t), filepath.Join(root, "dst"), filepath.Join(root, "dst"))
		u.left.setBrief(brief)
		u.setDensity("compact")
		var buf bytes.Buffer
		slog.SetDefault(slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})))
		u.left.navigate(big, "")
		press(u, fyne.KeyDown)
		out := buf.String()
		if lines := strings.Count(out, "\n"); lines > 6 {
			t.Errorf("brief=%v: %d debug records for one navigation and key press, want a small fixed number:\n%s", brief, lines, out)
		}
		for _, banned := range []string{"fill row", "grid update", "grid create", "list update", "list create", "list length", "row renderer", "list refresh", "grid refresh", "views replaced", "0xc0", "name=f0"} {
			if strings.Contains(out, banned) {
				t.Errorf("brief=%v: debug log contains %q", brief, banned)
			}
		}
		if strings.Contains(out, "content_width") != brief {
			t.Errorf("brief=%v: list state logs the wrong view's fields:\n%s", brief, out)
		}
		buf.Reset()
		slog.SetDefault(slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelInfo})))
		u.left.navigate(filepath.Join(root, "dst"), "")
		if buf.Len() != 0 {
			t.Errorf("brief=%v: navigation logged at Info level: %s", brief, buf.String())
		}
	}
}

func TestListStateLogsAroundNavigationAndKey(t *testing.T) {
	prev := slog.Default()
	t.Cleanup(func() { slog.SetDefault(prev) })
	root := fixture(t)
	u := newTestUI(t, test.NewTempApp(t), filepath.Join(root, "dst"), filepath.Join(root, "dst"))
	var buf bytes.Buffer
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})))
	u.left.navigate(filepath.Join(root, "src"), "")
	press(u, fyne.KeyDown)
	out := buf.String()
	for _, want := range []string{`msg="list state"`, "where=listing", "where=key", "entries=6", "cursor=1", "brief=false", "view_size=", "scroll="} {
		if !strings.Contains(out, want) {
			t.Errorf("list state log missing %q:\n%s", want, out)
		}
	}
}

// Brief view reports whether the cursor column is inside the scroller, the state that went wrong
// when a listing finished before the grid scrolled to the restored cursor.
func TestBriefListStateReportsCursorVisibility(t *testing.T) {
	prev := slog.Default()
	t.Cleanup(func() { slog.SetDefault(prev) })
	root := fixture(t)
	u := newTestUI(t, test.NewTempApp(t), filepath.Join(root, "dst"), filepath.Join(root, "dst"))
	u.left.setBrief(true)
	var buf bytes.Buffer
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})))
	logListState("probe", u.left)
	for _, want := range []string{"brief=true", "content_width=", "scroll=", "cursor_in_view=true"} {
		if !strings.Contains(buf.String(), want) {
			t.Errorf("brief list state missing %q:\n%s", want, buf.String())
		}
	}
}

// With debug logging off, a key press must not allocate for diagnostics: it runs on every
// held-arrow repeat.
func TestKeyPressDoesNotAllocateWithDebugOff(t *testing.T) {
	prev := slog.Default()
	t.Cleanup(func() { slog.SetDefault(prev) })
	root := fixture(t)
	u := newTestUI(t, test.NewTempApp(t), filepath.Join(root, "src"), filepath.Join(root, "dst"))
	slog.SetDefault(slog.New(slog.NewTextHandler(&bytes.Buffer{}, &slog.HandlerOptions{Level: slog.LevelInfo})))
	for _, brief := range []bool{false, true} {
		u.left.setBrief(brief)
		u.activate(u.left)
		e := &fyne.KeyEvent{Name: "Unmapped"}
		if n := testing.AllocsPerRun(500, func() { u.key(u.active, e, 20) }); n != 0 {
			t.Errorf("brief=%v: key press allocates %v times with debug off", brief, n)
		}
	}
}
