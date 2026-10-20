package commander

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"
)

func TestSpaceDirectorySize(t *testing.T) {
	for _, brief := range []bool{false, true} {
		root := fixture(t)
		if err := os.MkdirAll(filepath.Join(root, "src", "photos", "nested"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, "src", "photos", "nested", ".hidden"), []byte("12345"), 0o600); err != nil {
			t.Fatal(err)
		}
		u := newTestUI(t, test.NewTempApp(t), filepath.Join(root, "src"), root)
		p := u.left
		p.setBrief(brief)
		cursorTo(t, p, "photos")
		cursor := p.cursor
		press(u, fyne.KeySpace)
		if !p.marked["photos"] || p.cursor != cursor || p.status.Text != "0 B · 0 files · Calculating…" {
			t.Fatalf("Space changed marks/cursor or lost pending status: %q", p.status.Text)
		}
		waitUntil(t, func() bool { return !p.sizeBusy })
		if p.status.Text != "8 B · 2 files" {
			t.Fatalf("recursive file count: %q", p.status.Text)
		}
		p.markAll(false)
		cursorTo(t, p, "photos")
		if !p.statusRow.Visible() || p.status.Visible() {
			t.Fatalf("cached directory details: %q", p.status.Text)
		}
		cursorTo(t, p, "docs")
		press(u, fyne.KeySpace)
		waitUntil(t, func() bool { return !p.sizeBusy })
		cursorTo(t, p, "docs")
		if p.status.Text != "0 B · 0 files" {
			t.Fatalf("empty directory size: %q", p.status.Text)
		}
	}
}

func TestSpaceExitsTypeSearchForDirectorySize(t *testing.T) {
	for _, brief := range []bool{false, true} {
		for _, runeFirst := range []bool{false, true} {
			root := fixture(t)
			u := newTestUI(t, test.NewTempApp(t), filepath.Join(root, "src"), root)
			p := u.left
			p.setBrief(brief)
			u.activate(p)
			view := u.win.Canvas().Focused()
			test.Type(view, "ph")
			if p.current().name != "photos" || p.status.Text != "Search: ph" {
				t.Fatal("typing must still search")
			}
			cursor := p.cursor
			if runeFirst {
				view.TypedRune(' ')
			}
			view.TypedKey(&fyne.KeyEvent{Name: fyne.KeySpace})
			if !runeFirst {
				view.TypedRune(' ')
			}
			if p.searchPrefix != "" || p.cursor != cursor || !p.marked["photos"] || strings.HasPrefix(p.status.Text, "Search:") {
				t.Fatalf("Space must size without marking/moving/searching: prefix=%q cursor=%d status=%q", p.searchPrefix, p.cursor, p.status.Text)
			}
			waitUntil(t, func() bool { return !p.sizeBusy })
			if p.status.Text != "3 B · 1 files" {
				t.Fatalf("search hid directory size: %q", p.status.Text)
			}
			u.win.Canvas().Focus(p.pathBar)
			p.pathBar.SetText("")
			test.Type(p.pathBar, "folder name")
			if p.pathBar.Text != "folder name" {
				t.Fatal("Space must remain text inside path entry")
			}
		}
	}
}

func TestSpaceFolderMarksAndCombinedTotals(t *testing.T) {
	for _, brief := range []bool{false, true} {
		root := fixture(t)
		if err := os.WriteFile(filepath.Join(root, "src/docs/b.txt"), []byte("12345"), 0o600); err != nil {
			t.Fatal(err)
		}
		u := newTestUI(t, test.NewTempApp(t), filepath.Join(root, "src"), root)
		p := u.left
		p.setBrief(brief)
		for _, name := range []string{"docs", "photos"} {
			cursorTo(t, p, name)
			cursor := p.cursor
			press(u, fyne.KeySpace)
			if p.cursor != cursor || !p.marked[name] {
				t.Fatal("Space must mark folder without advancing")
			}
			r := newRowWidget(p, !brief)
			p.fillRow(p.cursor, r)
			var red bool
			for _, o := range laidOutObjects(r) {
				if label, ok := o.(*widget.Label); ok && label.Text == name && label.Importance == widget.DangerImportance {
					red = true
				}
			}
			if !red {
				t.Fatal("marked folder must use danger color")
			}
		}
		waitUntil(t, func() bool { return !p.sizeBusy })
		if p.status.Text != "8 B · 2 files" || p.statusRow.Visible() {
			t.Fatalf("combined totals only: %q", p.status.Text)
		}
		cursorTo(t, p, "readme.md")
		if p.status.Text != "8 B · 2 files" {
			t.Fatal("cursor navigation changed marked totals")
		}
		cursorTo(t, p, "photos")
		press(u, fyne.KeySpace)
		if p.marked["photos"] || p.status.Text != "5 B · 1 files" {
			t.Fatalf("unmark must remove folder from sum: %q", p.status.Text)
		}
		cursorTo(t, p, "docs")
		press(u, fyne.KeySpace)
		if len(p.marked) != 0 || !p.statusRow.Visible() {
			t.Fatal("last unmark must restore normal details")
		}
	}
}

func TestDirectorySizeWorkerBoundsAndStaleResult(t *testing.T) {
	root := fixture(t)
	u := newTestUI(t, test.NewTempApp(t), filepath.Join(root, "src"), root)
	p := u.left
	started := make(chan context.Context, 2)
	release := make(chan struct{})
	p.measureSize = func(ctx context.Context, _ []string, _ func(int64, int64)) (int64, int64, error) {
		started <- ctx
		<-release
		return 999, 9, nil
	}
	cursorTo(t, p, "docs")
	press(u, fyne.KeySpace)
	ctx := <-started
	for range 20 {
		cursorTo(t, p, "photos")
		press(u, fyne.KeySpace)
	}
	select {
	case <-started:
		t.Fatal("more than one size worker started")
	default:
	}
	// Reload the same path: generation, not path alone, must reject old results.
	p.refresh()
	if !errors.Is(ctx.Err(), context.Canceled) {
		t.Fatal("refresh did not cancel scan")
	}
	close(release)
	waitUntil(t, func() bool { return !p.sizeBusy })
	for _, e := range p.entries {
		if e.sizeKnown || e.sizeRequested {
			t.Fatal("stale size reached refreshed listing")
		}
	}
	select {
	case <-started:
		t.Fatal("stale queued request started after refresh")
	default:
	}
}

func TestDirectorySizeError(t *testing.T) {
	root := fixture(t)
	u := newTestUI(t, test.NewTempApp(t), filepath.Join(root, "src"), root)
	p := u.left
	p.measureSize = func(context.Context, []string, func(int64, int64)) (int64, int64, error) {
		return 123, 1, errors.New("permission denied")
	}
	cursorTo(t, p, "docs")
	press(u, fyne.KeySpace)
	waitUntil(t, func() bool { return !p.sizeBusy })
	if !strings.Contains(p.status.Text, "Folder size unavailable: permission denied") || strings.Contains(p.status.Text, "123 B") {
		t.Fatalf("partial size presented as complete: %q", p.status.Text)
	}
}

func TestDirectorySizeProgressThrottleAndQueue(t *testing.T) {
	now := time.Unix(0, 0)
	var queue []func()
	var got [2]int64
	report := directorySizeProgress(func() time.Time { return now }, latestUpdate(func(f func()) {
		queue = append(queue, f)
	}, func(value [2]int64) { got = value }))
	report(1, 1)
	now = now.Add(time.Second - time.Nanosecond)
	report(2, 2)
	if len(queue) != 0 {
		t.Fatal("progress refreshed before one second")
	}
	now = now.Add(time.Nanosecond)
	report(3, 3)
	for i := int64(4); i <= 100; i++ {
		now = now.Add(time.Second)
		report(i*10, i)
	}
	if len(queue) != 1 {
		t.Fatalf("progress queue unbounded: %d", len(queue))
	}
	queue[0]()
	if got != [2]int64{1000, 100} {
		t.Fatalf("latest totals lost: %v", got)
	}
}

func TestDirectorySizeProgressStatus(t *testing.T) {
	for _, brief := range []bool{false, true} {
		root := fixture(t)
		u := newTestUI(t, test.NewTempApp(t), filepath.Join(root, "src"), root)
		p := u.left
		p.setBrief(brief)
		cursorTo(t, p, "photos")
		e := &p.entries[p.cursor]
		e.sizeRequested, e.size, e.sizeFiles = true, 123, 4
		p.marked["photos"] = true
		p.updateStatus()
		if p.status.Text != "123 B · 4 files · Calculating…" {
			t.Fatalf("missing cursor progress: %q", p.status.Text)
		}
		p.marked["photos"] = true
		p.updateStatus()
		if p.status.Text != "123 B · 4 files · Calculating…" {
			t.Fatalf("partial totals must stay labelled as progress: %q", p.status.Text)
		}
		cursorTo(t, p, "docs")
		d := &p.entries[p.cursor]
		d.sizeRequested, d.sizeKnown, d.size, d.sizeFiles = true, true, 7, 2
		p.marked["docs"] = true
		p.updateStatus()
		if p.status.Text != "130 B · 6 files · Calculating…" {
			t.Fatalf("running and completed totals must sum: %q", p.status.Text)
		}
		cursorTo(t, p, "readme.md")
		cursor := p.cursor
		press(u, fyne.KeySpace)
		if p.cursor != cursor || len(p.marked) != 2 || !p.marked["photos"] || !p.marked["docs"] {
			t.Fatal("Space on file changed cursor or marks")
		}
	}
}
