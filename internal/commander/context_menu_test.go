package commander

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"testing"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"
)

func TestKeyboardContextMenu(t *testing.T) {
	for _, brief := range []bool{false, true} {
		root := fixture(t)
		u := newTestUI(t, test.NewTempApp(t), filepath.Join(root, "src"), filepath.Join(root, "src"))
		for _, p := range []*pane{u.left, u.right} {
			p.setBrief(brief)
			u.activate(p)
			cursorTo(t, p, "docs")
			p.marked["song.mp3"] = true
			cursor := p.cursor
			u.win.Canvas().Focused().TypedKey(&fyne.KeyEvent{Name: desktop.KeyMenu})
			popup, ok := u.win.Canvas().Focused().(*widget.PopUpMenu)
			if !ok || !popup.Visible() || len(popup.Items) != len(p.contextMenu().Items) {
				t.Fatal("Menu key did not open item context menu")
			}
			if p.cursor != cursor || !p.marked["song.mp3"] || u.active != p {
				t.Fatal("Menu key changed cursor, marks, or active pane")
			}
			popup.Hide()
			p.contextMenu().Items[0].Action()
			if p.path != filepath.Join(root, "src", "docs") {
				t.Fatal("context action did not target cursor folder")
			}
			p.entries = nil
			press(u, desktop.KeyMenu)
			if u.win.Canvas().Overlays().Top() != nil {
				t.Fatal("empty pane opened context menu")
			}
		}
	}
}

func TestExternalHandlerErrorDetails(t *testing.T) {
	if os.Getenv("DUALCOMMANDER_TEST_HANDLER") == "1" {
		_, _ = os.Stderr.WriteString("Finder: permission denied (-1743)")
		os.Exit(1)
	}
	root := fixture(t)
	u := newTestUI(t, test.NewTempApp(t), root, root)
	cmd := exec.Command(os.Args[0], "-test.run=^TestExternalHandlerErrorDetails$")
	cmd.Env = append(os.Environ(), "DUALCOMMANDER_TEST_HANDLER=1")
	u.startExternal(cmd, nil)
	waitUntil(t, func() bool { return hasWindow(u.app, "Error") })
	assertUIText(t, windowByTitle(t, u.app, "Error").Content(), "exit status 1: Finder: permission denied (-1743)")
	tapDialogButton(t, u, "OK")
}

type contextWindow struct {
	fyne.Window
	clipboard fyne.Clipboard
}

func (w *contextWindow) Clipboard() fyne.Clipboard { return w.clipboard }

func TestApplicationPanelWorkflow(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("macOS launcher stub uses executable shell script")
	}
	bin := t.TempDir()
	log := filepath.Join(bin, "arguments")
	if err := os.WriteFile(filepath.Join(bin, "open"), []byte("#!/bin/sh\nprintf '%s\\n' \"$@\" > \"$PANEL_TEST_LOG\"\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("PANEL_TEST_LOG", log)
	for _, application := range []string{"", "/Applications/Test App.app"} {
		root := fixture(t)
		u := newTestUI(t, test.NewTempApp(t), filepath.Join(root, "src"), root)
		p := u.active
		cursorTo(t, p, "readme.md")
		p.marked["song.mp3"] = true
		answer := make(chan string, 1)
		u.openWith(filepath.Join(p.path, "readme.md"), func() string { return <-answer })
		if u.passwordWindow == nil || u.win.Canvas().Focused() != nil {
			t.Fatal("parent not blocked during native application choice")
		}
		block, ok := u.win.Canvas().Overlays().Top().(*busyBlocker)
		if !ok || block.Size() != u.win.Canvas().Size() {
			t.Fatal("native chooser must block entire parent without waiting popup")
		}
		if block.Cursor() == nil {
			t.Fatal("blocked parent missing busy cursor")
		}
		for _, object := range test.LaidOutObjects(block) {
			if _, ok := object.(*widget.Label); ok {
				t.Fatal("blocker shows waiting text")
			}
		}
		test.Tap(block)
		test.TapSecondary(block)
		block.Scrolled(&fyne.ScrollEvent{})
		press(u, fyne.KeyDown)
		if p.current().name != "readme.md" {
			t.Fatal("parent navigation accepted while chooser open")
		}
		for _, menu := range u.win.MainMenu().Items {
			for _, item := range menu.Items {
				if !item.Disabled {
					t.Fatal("parent menu enabled during choice")
				}
			}
		}
		answer <- application
		waitUntil(t, func() bool { return u.passwordWindow == nil })
		if u.win.Canvas().Overlays().Top() != nil {
			t.Fatal("chooser left input blocker behind")
		}
		if u.win.Canvas().Focused() != p.focusable() || !p.marked["song.mp3"] || p.current().name != "readme.md" {
			t.Fatal("chooser did not restore parent focus and selection")
		}
		if application == "" {
			if _, err := os.Stat(log); !os.IsNotExist(err) {
				t.Fatal("cancelled choice launched application")
			}
		} else {
			var data []byte
			var err error
			want := "-a\n" + application + "\n--\n" + filepath.Join(p.path, "readme.md") + "\n"
			for deadline := time.Now().Add(5 * time.Second); time.Now().Before(deadline); {
				data, err = os.ReadFile(log)
				if err == nil && string(data) == want {
					break
				}
				time.Sleep(10 * time.Millisecond)
			}
			if err != nil || string(data) != "-a\n"+application+"\n--\n"+filepath.Join(p.path, "readme.md")+"\n" {
				t.Fatalf("wrong application launch: %q, %v", data, err)
			}
		}
	}
}

func TestContextMenuRename(t *testing.T) {
	for _, brief := range []bool{false, true} {
		for _, right := range []bool{false, true} {
			root := fixture(t)
			u := newTestUI(t, test.NewTempApp(t), filepath.Join(root, "src"), filepath.Join(root, "src"))
			p := u.left
			if right {
				p = u.right
			}
			p.setBrief(brief)
			p.marked["song.mp3"] = true
			for _, outcome := range []string{"escape", "close", "cancel", "submit"} {
				u.activate(u.other())
				r := newRowWidget(p, !brief)
				r.id = slices.Index(names(p), "readme.md")
				test.TapSecondary(r)
				popup, ok := u.win.Canvas().Focused().(*widget.PopUpMenu)
				if !ok {
					t.Fatal("right click did not show context menu")
				}
				popup.Hide()
				var rename *fyne.MenuItem
				for _, item := range p.contextMenu().Items {
					if item.Label == "Rename…" {
						rename = item
					}
				}
				if rename == nil || rename.Disabled {
					t.Fatal("right-click menu missing enabled Rename… action")
				}
				rename.Action()
				w := windowByTitle(t, u.app, "Rename")
				e := w.Canvas().Focused().(*escEntry)
				if e.Text != "readme.md" || u.passwordWindow == nil {
					t.Fatal("rename must prefill clicked file and block parent")
				}
				e.SetText("renamed.md")
				switch outcome {
				case "escape":
					e.TypedKey(&fyne.KeyEvent{Name: fyne.KeyEscape})
				case "close":
					u.inputWindow.Window.Close()
				case "cancel":
					tapDialogButton(t, u, "Cancel")
				case "submit":
					tapDialogButton(t, u, "Rename")
				}
				want := "readme.md"
				if outcome == "submit" {
					want = "renamed.md"
				}
				if exists(filepath.Join(p.path, "renamed.md")) != (outcome == "submit") || exists(filepath.Join(p.path, "readme.md")) != (outcome != "submit") {
					t.Fatal("rename confirmation/cancellation changed wrong file")
				}
				if p.current().name != want || !p.marked["song.mp3"] || !exists(filepath.Join(p.path, "song.mp3")) {
					t.Fatal("rename lost cursor or changed unrelated marked file")
				}
				if u.passwordWindow != nil || u.win.Canvas().Focused() != p.focusable() {
					t.Fatal("rename did not unblock parent and restore pane focus")
				}
			}
		}
	}
}

func TestRowContextMenu(t *testing.T) {
	for _, brief := range []bool{false, true} {
		root := fixture(t)
		u := newTestUI(t, test.NewTempApp(t), filepath.Join(root, "src"), filepath.Join(root, "src"))
		u.win = &contextWindow{Window: u.win, clipboard: test.NewClipboard()}
		p := u.right
		p.setBrief(brief)
		p.marked["song.mp3"] = true
		r := newRowWidget(p, !brief)
		r.id = slices.Index(names(p), "docs")
		test.TapSecondary(r)
		if u.active != p || p.current().name != "docs" || !p.marked["song.mp3"] {
			t.Fatal("right click lost target or marks")
		}
		popup, ok := u.win.Canvas().Focused().(*widget.PopUpMenu)
		if !ok {
			t.Fatal("right click did not show context menu")
		}
		if len(popup.Items) != len(p.contextMenu().Items) || !popup.Visible() {
			t.Fatal("context menu structure differs from actions")
		}
		popup.Hide()
		menu := p.contextMenu()
		for _, item := range menu.Items {
			if item.Label == "Copy Path" {
				item.Action()
				if got := u.win.Clipboard().Content(); got != filepath.Join(p.path, "docs") {
					t.Fatalf("copied marked file instead of clicked folder: %q", got)
				}
			}
			if item.Label == "Open With…" && item.Disabled != (runtime.GOOS != "darwin") {
				t.Fatal("wrong folder application chooser availability")
			}
		}
		menu.Items[0].Action()
		if p.path != filepath.Join(root, "src", "docs") {
			t.Fatal("Open did not enter clicked folder")
		}
		p.cd(filepath.Join(root, "src"), "readme.md")
		menu = p.contextMenu()
		if runtime.GOOS == "darwin" || runtime.GOOS == "windows" {
			if menu.Items[1].Label != "Open With…" || menu.Items[1].Disabled || menu.Items[2].Disabled {
				t.Fatal("native file actions missing")
			}
		}
		// Virtual members must never reach OS handlers.
		p.path = filepath.Join(root, "src", "app.zip", "docs")
		for _, item := range p.contextMenu().Items[1:] {
			if !item.IsSeparator && !item.Disabled {
				t.Fatalf("archive action enabled: %s", item.Label)
			}
		}
		p.path = filepath.Join(root, "src")
		cursorTo(t, p, "..")
		for _, item := range p.contextMenu().Items {
			if !item.IsSeparator && !item.Disabled {
				t.Fatalf("parent entry action enabled: %s", item.Label)
			}
		}
		r.id = -1
		test.TapSecondary(r)
		if u.win.Canvas().Overlays().Top() != nil {
			t.Fatal("empty cell opened menu")
		}
		r.id = 1
		u.passwordWindow = u.app.NewWindow("Blocked")
		test.TapSecondary(r)
		if u.win.Canvas().Overlays().Top() != nil {
			t.Fatal("blocked parent opened context menu")
		}
		u.passwordWindow.Close()
		u.passwordWindow = nil
	}
}
