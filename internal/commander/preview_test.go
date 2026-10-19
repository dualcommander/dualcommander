package commander

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"
)

func previewFixture(t *testing.T) []byte {
	t.Helper()
	im := image.NewRGBA(image.Rect(0, 0, 32, 32))
	for y := 0; y < 32; y++ {
		for x := 0; x < 32; x++ {
			im.Set(x, y, color.RGBA{R: uint8(x * 8), G: uint8(y * 8), B: 100, A: 255})
		}
	}
	var b bytes.Buffer
	if err := png.Encode(&b, im); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

func TestPreviewsOnlyInViewport(t *testing.T) {
	for _, brief := range []bool{false, true} {
		t.Run(fmt.Sprintf("brief=%v", brief), func(t *testing.T) {
			root := t.TempDir()
			for i := 0; i < 100; i++ {
				if err := os.WriteFile(filepath.Join(root, fmt.Sprintf("file-%03d.png", i)), nil, 0o600); err != nil {
					t.Fatal(err)
				}
			}
			u := newTestUI(t, test.NewTempApp(t), root, root)
			p := u.left
			p.setBrief(brief)
			w := test.NewWindow(p.body)
			defer w.Close()
			w.Resize(fyne.NewSize(600, 130))
			called := 0
			u.filePreview = func(string) []byte {
				post(func() { called++ })
				return nil
			}
			// Simulate UpdateItem on all buffer rows: only intersecting rows load.
			for i := range p.entries {
				p.entryIcon(i)
			}
			requested := 0
			for _, e := range p.entries {
				if e.iconRequested {
					requested++
				}
			}
			if requested == 0 || requested >= 30 || p.entries[len(p.entries)-1].iconRequested {
				t.Fatalf("not lazy: %d of %d requested", requested, len(p.entries))
			}
			waitUntil(t, func() bool { return called == requested })
			u.filePreview = nil
			// End key scrolls the last item into view in either layout.
			press(u, fyne.KeyEnd)
			last := len(p.entries) - 1
			if !p.previewVisible(last) || p.previewVisible(1) {
				t.Fatal("viewport did not follow scrolling")
			}
			data := previewFixture(t)
			u.filePreview = func(string) []byte { return data }
			p.entryIcon(last)
			u.filePreview = nil
			if !p.entries[last].iconRequested {
				t.Fatal("newly visible file not requested")
			}
			waitUntil(t, func() bool { return p.entries[last].icon != nil })
		})
	}
}

func TestQueuedPreviewSkipsScrolledOutFile(t *testing.T) {
	root := t.TempDir()
	for i := 0; i < 40; i++ {
		if err := os.WriteFile(filepath.Join(root, fmt.Sprintf("file-%03d.png", i)), nil, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	u := newTestUI(t, test.NewTempApp(t), root, root)
	p := u.left
	w := test.NewWindow(p.body)
	defer w.Close()
	w.Resize(fyne.NewSize(600, 130))
	// Reserve worker slots before queuing a visible file.
	previewWorkers <- struct{}{}
	previewWorkers <- struct{}{}
	var called atomic.Int32
	u.filePreview = func(string) []byte { called.Add(1); return nil }
	p.entryIcon(1)
	u.filePreview = nil
	if !p.entries[1].iconRequested {
		t.Fatal("visible preview not queued")
	}
	press(u, fyne.KeyEnd)
	<-previewWorkers
	<-previewWorkers
	waitUntil(t, func() bool { return !p.entries[1].iconRequested })
	if called.Load() != 0 {
		t.Fatal("scrolled-out queued file reached native loader")
	}
	press(u, fyne.KeyHome)
	data := previewFixture(t)
	u.filePreview = func(string) []byte { return data }
	p.entryIcon(1)
	u.filePreview = nil
	if !p.entries[1].iconRequested {
		t.Fatal("skipped file not eligible when visible again")
	}
	waitUntil(t, func() bool { return p.entries[1].icon != nil })
}

func TestFilePreviewRows(t *testing.T) {
	root := fixture(t)
	u := newTestUI(t, test.NewTempApp(t), filepath.Join(root, "src"), root)
	data := previewFixture(t)
	var calls atomic.Int32
	cursorTo(t, u.left, "readme.md")
	u.filePreview = func(string) []byte { calls.Add(1); return data }
	i := u.left.cursor
	fallback := u.left.entryIcon(i)
	if fallback != iconFor(u.left.entries[i]) {
		t.Fatal("pending preview did not use type icon")
	}
	u.left.entryIcon(i)
	u.filePreview = nil
	waitUntil(t, func() bool { return u.left.entries[i].icon != nil })
	if calls.Load() != 1 || !bytes.Equal(u.left.entryIcon(i).Content(), data) {
		t.Fatal("preview not cached per row")
	}
	// Directories never request thumbnails.
	cursorTo(t, u.left, "docs")
	if u.left.entryIcon(u.left.cursor) != icDir {
		t.Fatal("directory icon changed")
	}
	u.filePreview = nil
	applyTheme(u.app, "light", u.density)
	u.left.entries[0].attrs = "drwxr-xr-x"
	u.left.entries[0].mod = u.left.entries[i].mod
	u.left.list.Refresh()
	w := test.NewWindow(u.left.body)
	defer w.Close()
	w.Resize(fyne.NewSize(480, 280))
	assertPaneRows(t, u.left)
	for _, o := range test.LaidOutObjects(w.Content()) {
		if icon, ok := o.(*widget.Icon); ok && icon.Resource != nil && bytes.Equal(icon.Resource.Content(), data) {
			return
		}
	}
	t.Fatal("loaded preview missing from laid-out row")
}

func TestFilePreviewStaleAndFallback(t *testing.T) {
	root := fixture(t)
	u := newTestUI(t, test.NewTempApp(t), filepath.Join(root, "src"), root)
	data := previewFixture(t)
	started, release := make(chan struct{}), make(chan struct{})
	cursorTo(t, u.left, "readme.md")
	u.filePreview = func(string) []byte {
		close(started)
		post(func() {})
		<-release
		return data
	}
	u.left.entryIcon(u.left.cursor)
	u.filePreview = nil
	waitUntil(t, func() bool {
		select {
		case <-started:
			return true
		default:
			return false
		}
	})
	u.filePreview = nil
	u.left.cd(filepath.Join(root, "dst"), "")
	close(release)
	// Drain queued completion; newer listing must remain untouched.
	select {
	case f := <-uiQueue:
		f()
	case <-time.After(10 * time.Second):
		t.Fatal("timed out waiting for preview completion")
	}
	for _, e := range u.left.entries {
		if e.icon != nil {
			t.Fatal("stale preview attached to new directory")
		}
	}
	u.left.cd(filepath.Join(root, "src"), "readme.md")
	u.filePreview = func(string) []byte { return nil }
	i := u.left.cursor
	want := iconFor(u.left.entries[i])
	if u.left.entryIcon(i) != want {
		t.Fatal("missing preview did not preserve fallback")
	}
	u.filePreview = nil
}
