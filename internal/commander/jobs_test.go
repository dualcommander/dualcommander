package commander

import (
	"context"
	"strings"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"
	"github.com/dualcommander/dualcommander/pkg/fsops"
)

func TestOperationsFileCounts(t *testing.T) {
	root := fixture(t)
	u := newTestUI(t, test.NewTempApp(t), root, root)
	u.jobs.runCounted("Copy", nil, func(ctx context.Context, report func(fsops.CopyProgress)) error {
		report(fsops.CopyProgress{Done: 25, Total: 100, FilesDone: 2, FilesTotal: 7, Current: "/source/file.txt"})
		<-ctx.Done()
		return ctx.Err()
	})
	waitUntil(t, func() bool { return u.jobs.active[0].filesTotal == 7 })
	if !strings.Contains(u.jobs.button.Text, "25% · 2/7 files") {
		t.Fatal(u.jobs.button.Text)
	}
	u.jobs.show()
	row := u.jobs.list.CreateItem()
	u.jobs.list.UpdateItem(0, row)
	assertUIText(t, row, "file.txt · 25% · 2/7 files")
	u.jobs.active[0].cancel()
	waitJobs(t, u)
	u.jobs.monitor.Close()
}

func TestOperationsMonitor(t *testing.T) {
	for _, dismissal := range []string{"Close", "Escape", "native"} {
		t.Run(dismissal, func(t *testing.T) {
			root := fixture(t)
			u := newTestUI(t, test.NewTempApp(t), root, root)
			stopped := make(chan struct{})
			u.jobs.run("Copy", nil, func(ctx context.Context, progress fsops.Progress) error {
				progress(25, 100, "/source/file.txt")
				<-ctx.Done()
				close(stopped)
				return ctx.Err()
			})
			waitUntil(t, func() bool { return u.jobs.active[0].total == 100 })
			if u.jobs.bar.Value != .25 || !strings.Contains(u.jobs.button.Text, "25%") {
				t.Fatal("single-task progress missing")
			}
			test.Tap(u.jobs.button)
			w := windowByTitle(t, u.app, "Running Operations")
			if u.passwordWindow != w || u.win.Canvas().Overlays().Top() == nil || u.jobs.list.Length() != 1 {
				t.Fatal("monitor must block parent and show scrollable task list")
			}
			path := u.active.path
			press(u, fyne.KeyBackspace)
			if u.active.path != path {
				t.Fatal("parent accepted keys")
			}
			switch dismissal {
			case "Close":
				w.Canvas().Focused().TypedKey(&fyne.KeyEvent{Name: fyne.KeyReturn})
			case "Escape":
				u.jobs.list.TypedKey(&fyne.KeyEvent{Name: fyne.KeyEscape})
			case "native":
				w.Close()
			}
			if u.passwordWindow != nil || u.jobs.monitor != nil || u.win.Canvas().Focused() != u.active.focusable() {
				t.Fatal("dismissal did not restore parent")
			}
			select {
			case <-stopped:
				t.Fatal("monitor close cancelled operation")
			default:
			}
			test.Tap(u.jobs.button)
			w = u.jobs.monitor
			row := u.jobs.list.CreateItem()
			u.jobs.list.UpdateItem(0, row)
			assertUIText(t, row, "Copy", "file.txt · 25%")
			// Escape from per-task button dismisses monitoring, not work.
			stop := row.(*fyne.Container).Objects[1].(*dialogButton)
			if stop.Text != "Cancel" || stop.Disabled() {
				t.Fatal("missing task cancellation control")
			}
			stop.TypedKey(&fyne.KeyEvent{Name: fyne.KeyEscape})
			if u.jobs.monitor != nil {
				t.Fatal("task button ignored Escape")
			}
			u.jobs.show()
			row = u.jobs.list.CreateItem()
			u.jobs.list.UpdateItem(0, row)
			row.(*fyne.Container).Objects[1].(*dialogButton).Tapped(nil)
			waitJobs(t, u)
			if u.jobs.box.Visible() || len(u.jobs.active) != 0 {
				t.Fatal("idle strip still visible")
			}
			u.jobs.monitor.Close()
		})
	}
}

func TestOperationsMixedProgress(t *testing.T) {
	root := fixture(t)
	u := newTestUI(t, test.NewTempApp(t), root, root)
	finish := make(chan struct{})
	defer close(finish)
	for range 12 {
		u.jobs.run("Delete", nil, func(ctx context.Context, progress fsops.Progress) error {
			select {
			case <-finish:
				return nil
			case <-ctx.Done():
				return ctx.Err()
			}
		})
	}
	if !u.jobs.activity.Visible() || u.jobs.bar.Visible() || strings.Contains(u.jobs.button.Text, "%") {
		t.Fatal("mixed/unknown work must not show percentage")
	}
	u.jobs.show()
	if u.jobs.list.Length() != 12 || u.jobs.monitor.Content().Size().Height <= 0 {
		t.Fatal("missing scrollable operations")
	}
	u.jobs.monitor.Canvas().Unfocus()
	u.jobs.monitor.Canvas().OnTypedKey()(&fyne.KeyEvent{Name: fyne.KeyEscape})
	for _, op := range u.jobs.active {
		op.cancel()
	}
	waitJobs(t, u)
}

func TestOperationsStripThinTrack(t *testing.T) {
	root := fixture(t)
	u := newTestUI(t, test.NewTempApp(t), root, root)
	j := u.jobs
	j.active = []*operation{{title: "Copy", done: 25, total: 100}}
	defer func() { j.active = nil; j.update() }()
	for _, total := range []int64{100, 0} {
		j.active[0].total = total
		j.update()
		strip := j.box.Objects[0].(*fyne.Container)
		for _, width := range []float32{320, 800} {
			strip.Resize(fyne.NewSize(width, strip.MinSize().Height))
			if j.bar.Size().Height != 3 || j.activity.Size().Height != 3 {
				t.Fatal("determinate and activity tracks must stay 3 px tall")
			}
			if j.bar.Size().Width != width || j.activity.Size().Width != width ||
				j.button.Size().Height < j.button.MinSize().Height ||
				j.bar.Position().Y != j.button.Size().Height {
				t.Fatal("thin track must span strip below full-sized click target")
			}
		}
	}
}

func TestOperationsDialogThinTrack(t *testing.T) {
	root := fixture(t)
	u := newTestUI(t, test.NewTempApp(t), root, root)
	j := u.jobs
	j.active = []*operation{{title: "Copy", file: "/source/file.txt", done: 25, total: 100}}
	j.update()
	j.show()
	defer func() { j.closeMonitor(); j.active = nil; j.update() }()
	row := j.list.CreateItem().(*fyne.Container)
	for _, total := range []int64{100, 0, 200} {
		j.active[0].total = total
		j.list.UpdateItem(0, row)
		for _, width := range []float32{320, 800} {
			row.Resize(fyne.NewSize(width, row.MinSize().Height))
			fields := row.Objects[0].(*fyne.Container)
			track := fields.Objects[2].(*fyne.Container)
			bar := track.Objects[0].(*widget.ProgressBar)
			if track.MinSize().Height != 3 || bar.Size().Height != 3 || bar.Size().Width != fields.Size().Width {
				t.Fatal("dialog progress must remain full-width and 3 px tall")
			}
			if bar.TextFormatter() != "" || bar.Visible() != (total > 0) {
				t.Fatal("track must not contain text or show unknown percentage")
			}
			if total > 0 && bar.Value != float64(25)/float64(total) {
				t.Fatal("progress value lost when recycling row")
			}
			stop := row.Objects[1].(*dialogButton)
			if stop.Size().Height < stop.MinSize().Height {
				t.Fatal("thin progress reduced Cancel target")
			}
		}
	}
}
