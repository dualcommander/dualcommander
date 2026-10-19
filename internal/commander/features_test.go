package commander

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"
)

// Density menu: compact rows shorter than standard; persisted; cursor survives rebuild.
func TestDensityChangesRowHeight(t *testing.T) {
	root := fixture(t)
	a := test.NewTempApp(t)
	u := newTestUI(t, a, filepath.Join(root, "src"), root)
	cursorTo(t, u.left, "readme.md")
	if !slices.Equal(densities, []string{"compact", "standard"}) || len(u.densItems) != 2 {
		t.Fatal("density menu must contain only Compact and Standard")
	}
	h := map[string]float32{}
	for _, d := range densities {
		u.setDensity(d)
		h[d] = newRowWidget(u.left, true).MinSize().Height
	}
	if !(h["compact"] < h["standard"]) {
		t.Errorf("row heights %v", h)
	}
	if a.Preferences().String(prefDensity) != "standard" || u.left.current().name != "readme.md" {
		t.Errorf("pref %q cursor %q", a.Preferences().String(prefDensity), u.left.current().name)
	}
}

func TestDensityMigration(t *testing.T) {
	root := fixture(t)
	for _, saved := range []string{"", "cozy", "roomy", "standard", "compact"} {
		t.Run(saved, func(t *testing.T) {
			a := test.NewTempApp(t)
			a.Preferences().SetString(prefDensity, saved)
			u := newTestUI(t, a, root, root)
			want := "standard"
			if saved == "compact" {
				want = saved
			}
			u.save()
			if u.density != want || a.Preferences().String(prefDensity) != want {
				t.Fatalf("saved %q: density=%q want %q", saved, u.density, want)
			}
		})
	}
}

func TestEntryNamesNotBold(t *testing.T) {
	root := fixture(t)
	u := newTestUI(t, test.NewTempApp(t), filepath.Join(root, "src"), root)
	for _, detailed := range []bool{false, true} {
		r := newRowWidget(u.left, detailed)
		for i := range u.left.entries {
			u.left.fillRow(i, r)
			content := r.content.(*fyne.Container)
			if !detailed {
				content = content.Objects[0].(*fyne.Container)
			}
			if content.Objects[0].(*widget.Label).TextStyle.Bold {
				t.Fatalf("bold entry %q, detailed=%v", u.left.entries[i].name, detailed)
			}
		}
	}
}

// Detailed view sorts by name/size/date via header clicks; dirs stay on top; ".." first.
func TestSortByDateSizeName(t *testing.T) {
	root := fixture(t)
	src := filepath.Join(root, "src")
	day := func(d int) time.Time { return time.Date(2024, 1, d, 0, 0, 0, 0, time.UTC) }
	for n, d := range map[string]int{"readme.md": 3, "song.mp3": 1, "app.zip": 2, "docs": 5, "photos": 4} {
		os.Chtimes(filepath.Join(src, n), day(d), day(d))
	}
	a := test.NewTempApp(t)
	u := newTestUI(t, a, src, root)
	p := u.left
	cursorTo(t, p, "app.zip")
	for _, c := range []struct {
		click string
		want  string
	}{
		{"date", ".. photos docs song.mp3 app.zip readme.md"}, // ascending; dirs sorted among themselves
		{"date", ".. docs photos readme.md app.zip song.mp3"}, // same column again: descending
		{"size", ".. docs photos app.zip song.mp3 readme.md"}, // dirs by name; 2B, 2B (name tiebreak), 5B
		{"name", ".. docs photos app.zip readme.md song.mp3"},
	} {
		test.Tap(p.headers[slices.Index(sortKeys, c.click)])
		if got := strings.Join(names(p), " "); got != c.want {
			t.Errorf("after %s: %s want %s", c.click, got, c.want)
		}
	}
	if p.current().name != "app.zip" {
		t.Errorf("cursor moved to %q", p.current().name)
	}
	// persisted per pane
	test.Tap(p.headers[2])
	test.Tap(p.headers[2])
	if got := a.Preferences().String(prefLeftSort); got != "date desc" {
		t.Errorf("pref %q", got)
	}
	if q := newPane(u, src, false, "date desc"); !q.desc || q.sortBy != "date" || !strings.Contains(q.headers[2].Text, "▾") {
		t.Errorf("restore: %s %v %q", q.sortBy, q.desc, q.headers[2].Text)
	}
}

func TestBriefSortHeader(t *testing.T) {
	root := fixture(t)
	u := newTestUI(t, test.NewTempApp(t), filepath.Join(root, "src"), root)
	p := u.left
	p.setBrief(true)
	w := test.NewWindow(p.body)
	defer w.Close()
	w.Resize(fyne.NewSize(800, 280))
	cursorTo(t, p, "readme.md")
	p.marked["readme.md"] = true
	for i, key := range sortKeys {
		// Choose another key first so every first click starts ascending.
		p.setSort(sortKeys[(i+1)%len(sortKeys)])
		for _, desc := range []bool{false, true} {
			test.Tap(p.headers[i])
			if p.hdr.Size().Height <= 0 || p.grid.Position().Y < p.hdr.Size().Height {
				t.Fatal("brief sorting header not above grid")
			}
			if p.sortBy != key || p.desc != desc || !strings.HasSuffix(p.headers[i].Text, arrow(true, desc)) {
				t.Fatalf("brief sort %s descending=%v not applied", key, desc)
			}
			want := slices.Clone(p.entries)
			sortEntries(want, key, desc)
			for j := range want {
				if want[j].name != p.entries[j].name {
					t.Fatalf("brief entries not sorted by %s", key)
				}
			}
			if !p.brief || p.current().name != "readme.md" || !p.marked["readme.md"] || u.win.Canvas().Focused() != p.grid {
				t.Fatal("brief sorting lost view, cursor, mark, or focus")
			}
		}
	}
	p.setBrief(false)
	if p.sortBy != "date" || !p.desc {
		t.Fatal("view switch lost sort")
	}
}

func TestStartupSelectsFirstEntryInBothPanes(t *testing.T) {
	root := fixture(t)
	u := newTestUI(t, test.NewTempApp(t), filepath.Join(root, "src"), filepath.Join(root, "src", "photos"))
	for _, c := range []struct {
		p    *pane
		want string
	}{{u.left, "docs"}, {u.right, "a.png"}} {
		if got := c.p.current().name; got != c.want {
			t.Errorf("cursor %q want %q", got, c.want)
		}
	}
	for _, active := range []*pane{u.left, u.right} {
		if u.active != active || u.win.Canvas().Focused() != active.focusable() {
			t.Fatal("active pane does not have keyboard focus")
		}
		for _, p := range []*pane{u.left, u.right} {
			if p.pathBar.TextStyle.Bold != (p == active) {
				t.Fatal("active pane indicator incorrect")
			}
			assertPaneRows(t, p)
			assertUIText(t, p.body, p.current().name)
		}
		press(u, fyne.KeyTab)
	}
	if u.active != u.left || u.left.current().name != "docs" {
		t.Errorf("tab back: active left=%v cursor %q", u.active == u.left, u.left.current().name)
	}
}

func TestAutoRefreshOnExternalChange(t *testing.T) {
	root := fixture(t)
	u := newTestUI(t, test.NewTempApp(t), filepath.Join(root, "src"), root)
	os.WriteFile(filepath.Join(root, "src", "zz.txt"), []byte("x"), 0o644)
	waitUntil(t, func() bool { return slices.Contains(names(u.left), "zz.txt") })
	os.Remove(filepath.Join(root, "src", "zz.txt"))
	waitUntil(t, func() bool { return !slices.Contains(names(u.left), "zz.txt") })
}

// Mixed keyboard + mouse: mouse-down must act immediately on any row, incl. the cursor row.
func TestMouseDownAfterKeyboard(t *testing.T) {
	root := fixture(t)
	u := newTestUI(t, test.NewTempApp(t), filepath.Join(root, "src"), filepath.Join(root, "dst"))
	p := u.left
	down := func(n string, m fyne.KeyModifier) {
		r := newRowWidget(p, true)
		p.fillRow(slices.Index(names(p), n), r)
		r.MouseDown(&desktop.MouseEvent{Button: desktop.MouseButtonPrimary, Modifier: m})
	}
	press(u, fyne.KeyDown) // keyboard: docs -> photos
	down("photos", fyne.KeyModifierShortcutDefault)
	down("readme.md", 0) // second row right after: no double-tap swallowing
	if p.current().name != "readme.md" || !p.marked["photos"] {
		t.Fatalf("cursor %q marked %v", p.current().name, p.marked)
	}
	press(u, fyne.KeyUp)
	down("app.zip", fyne.KeyModifierShortcutDefault) // Mod-click on cursor row toggles
	if !p.marked["app.zip"] || p.current().name != "app.zip" {
		t.Fatalf("cursor %q marked %v", p.current().name, p.marked)
	}
	u.activate(u.right)
	down("song.mp3", 0) // click in inactive pane activates it
	if u.active != p || p.current().name != "song.mp3" {
		t.Fatal("click did not activate pane")
	}
}

// keyboard via real focus path (focused widget, else canvas handler), like the glfw driver.
func TestKeyboardFocusPath(t *testing.T) {
	root := fixture(t)
	u := newTestUI(t, test.NewTempApp(t), filepath.Join(root, "src"), filepath.Join(root, "dst"))
	p := u.left
	c := u.win.Canvas()
	typ := func(k fyne.KeyName) {
		e := &fyne.KeyEvent{Name: k}
		if f := c.Focused(); f != nil {
			f.TypedKey(e)
		} else {
			c.OnTypedKey()(e)
		}
	}
	at := func(want string) {
		t.Helper()
		if got := p.current().name; got != want {
			t.Fatalf("cursor %q want %q", got, want)
		}
	}
	// names: .. docs photos app.zip readme.md song.mp3
	typ(fyne.KeyDown)
	at("photos")
	typ(fyne.KeyLeft) // detailed: no-op
	at("photos")
	test.Tap(p.headers[1]) // header click steals focus; keys must keep working
	at("photos")
	typ(fyne.KeyEnd)
	typ(fyne.KeyUp)
	at("song.mp3") // size sort: .. docs photos app.zip song.mp3 readme.md
	c.Unfocus()    // e.g. after dialog: canvas fallback
	typ(fyne.KeyHome)
	at("..")
	typ(fyne.KeyTab)
	if u.active != u.right || c.Focused() != u.right.focusable() {
		t.Fatal("tab did not move focus")
	}
	typ(fyne.KeyTab)
	p.setBrief(true) // brief: Up/Down step vertically, Left/Right jump columns
	u.activate(p)
	p.grid.Resize(fyne.NewSize(800, 60))
	cols := p.cols()
	if cols < 2 {
		t.Fatal("brief navigation test needs multiple columns")
	}
	typ(fyne.KeyRight)
	if p.cursor != p.grid.rows {
		t.Fatalf("right in brief: %d rows %d", p.cursor, p.grid.rows)
	}
	typ(fyne.KeyLeft)
	typ(fyne.KeyDown)
	if p.cursor != min(1, p.grid.rows-1) {
		t.Fatalf("down in brief: %d cols %d", p.cursor, cols)
	}
}

func TestMouseMultiSelectThenCopy(t *testing.T) {
	root := fixture(t)
	u := newTestUI(t, test.NewTempApp(t), filepath.Join(root, "src"), filepath.Join(root, "dst"))
	p := u.left
	idx := func(n string) int { return slices.Index(names(p), n) }
	p.click(idx(".."), 0)
	p.click(idx("photos"), fyne.KeyModifierShift) // range: docs, photos (".." skipped)
	p.click(idx("song.mp3"), fyne.KeyModifierShortcutDefault)
	p.click(idx("docs"), fyne.KeyModifierShortcutDefault) // toggle off
	got := p.selection()
	want := []string{filepath.Join(p.path, "photos"), filepath.Join(p.path, "song.mp3")}
	if !slices.Equal(got, want) {
		t.Fatalf("got %v want %v", got, want)
	}
	press(u, fyne.KeyF5)
	tapDialogButton(t, u, "Copy")
	waitJobs(t, u)
	if !exists(filepath.Join(root, "dst", "photos", "a.png")) || !exists(filepath.Join(root, "dst", "song.mp3")) {
		t.Fatal("multi copy failed")
	}
}

func TestEditCmd(t *testing.T) {
	for goos, want := range map[string]string{
		"darwin": "open -t /d/a b.txt", "windows": "notepad /d/a b.txt", "linux": "xdg-open /d/a b.txt",
	} {
		c := editCmd(goos, "/d/a b.txt")
		if got := strings.Join(c, " "); got != want || c[len(c)-1] != "/d/a b.txt" { // path one arg
			t.Errorf("%s: %q", goos, got)
		}
	}
}

func TestSearchCmd(t *testing.T) {
	for _, c := range []struct {
		goos       string
		rg         bool
		text, glob string
		want       string
	}{
		{"darwin", true, "hi", "", "rg -l -i -F -uu --iglob * -e hi /d"},
		{"linux", true, "", "*.go", "rg --files -uu --iglob *.go /d"},
		{"darwin", false, "hi", "*.md", "grep -rlIiF --include=*.md -e hi /d"},
		{"linux", false, "", "*.md", "find /d -iname *.md"},
		{"windows", false, "hi", "", "findstr /s /m /i /l /c:hi " + filepath.Join("/d", "*")},
		{"windows", false, "", "*.md", "where /r /d *.md"},
	} {
		if got := searchCmd(c.goos, c.rg, "/d", c.text, c.glob); strings.Join(got, " ") != c.want {
			t.Errorf("%+v: got %q", c, got)
		}
	}
}

// Runs the real native tool(s) present on this machine.
func TestRunSearchNative(t *testing.T) {
	root := fixture(t)
	_, rgErr := exec.LookPath("rg")
	for _, rg := range []bool{false, rgErr == nil} {
		var got []string
		runSearch(context.Background(), searchCmd(runtime.GOOS, rg, root, "HELLO", ""), func(b []string) { got = append(got, b...) })
		if !slices.Equal(got, []string{filepath.Join(root, "src", "readme.md")}) {
			t.Errorf("rg=%v text: %v", rg, got)
		}
		got = nil
		runSearch(context.Background(), searchCmd(runtime.GOOS, rg, root, "", "*.PNG"), func(b []string) { got = append(got, b...) })
		if !slices.Equal(got, []string{filepath.Join(root, "src", "photos", "a.png")}) {
			t.Errorf("rg=%v name: %v", rg, got)
		}
	}
}

func TestSearchDialogJumpsToResult(t *testing.T) {
	root := fixture(t)
	u := newTestUI(t, test.NewTempApp(t), root, root)
	f := u.search()
	c := f.win.Canvas()
	if c.Focused() != f.text {
		t.Fatal("search text not focused")
	}
	test.Type(f.text, "hello")
	f.text.TypedKey(&fyne.KeyEvent{Name: fyne.KeyReturn})
	waitUntil(t, func() bool { return f.done })
	if len(f.results) != 1 {
		t.Fatalf("results: %v", f.results)
	}
	f.list.Select(0)
	if u.left.path != filepath.Join(root, "src") || u.left.current().name != "readme.md" {
		t.Fatalf("jump: %s %q", u.left.path, u.left.current().name)
	}
	if slices.Contains(u.app.Driver().AllWindows(), f.win) {
		t.Fatal("window not closed")
	}
}

// Esc closes Find Files from a focused entry and with nothing focused; typing still works.
func TestSearchEscCloses(t *testing.T) {
	root := fixture(t)
	u := newTestUI(t, test.NewTempApp(t), root, root)
	var f *finder
	open := func() bool { return slices.Contains(u.app.Driver().AllWindows(), f.win) }
	esc := &fyne.KeyEvent{Name: fyne.KeyEscape}
	f = u.search()
	test.Type(f.glob, "x")
	if f.glob.Text != "x" || !open() {
		t.Fatal("typing broken")
	}
	f.glob.TypedKey(esc)
	if open() {
		t.Fatal("esc in entry did not close")
	}
	f = u.search()
	f.win.Canvas().Unfocus()
	f.win.Canvas().OnTypedKey()(esc)
	if open() {
		t.Fatal("esc with no focus did not close")
	}
}
