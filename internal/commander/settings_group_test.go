package commander

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
)

func TestDiskSpaceSetting(t *testing.T) {
	root := fixture(t)
	a := test.NewTempApp(t)
	u := newTestUI(t, a, root, root)
	path := filepath.Join(t.TempDir(), "settings.json")
	store, err := openSettingsFile(a.Preferences(), path, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.close() })
	for _, free := range []bool{true, false, true} {
		u.settings()
		w := windowByTitle(t, a, "Settings")
		var toggle *dialogCheck
		for _, object := range test.LaidOutObjects(w.Content()) {
			if c, ok := object.(*dialogCheck); ok && c.Text == "Show free disk space" {
				toggle = c
			}
		}
		if toggle == nil || toggle.Checked != free {
			t.Fatalf("free-space setting missing or not restored: want %v", free)
		}
		for _, p := range []*pane{u.left, u.right} {
			suffix := " free / "
			if !free {
				suffix = " used / "
			}
			waitUntil(t, func() bool { return strings.Contains(p.space.Text, suffix) })
		}
		test.Tap(toggle)
		if a.Preferences().Bool("showFreeSpace") != !free {
			t.Fatal("disk space choice not saved immediately")
		}
		w.Canvas().Focus(toggle)
		toggle.TypedKey(&fyne.KeyEvent{Name: fyne.KeyEscape})
		if hasWindow(a, "Settings") {
			t.Fatal("Escape left Settings open")
		}
		if err := store.save(); err != nil {
			t.Fatal(err)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		var document struct {
			Views struct {
				ShowFreeSpace *bool `json:"showFreeSpace"`
			} `json:"views"`
		}
		if err := json.Unmarshal(data, &document); err != nil {
			t.Fatal(err)
		}
		if document.Views.ShowFreeSpace == nil || *document.Views.ShowFreeSpace != !free {
			t.Fatal("views.showFreeSpace not persisted", string(data))
		}
		prefs := test.NewTempApp(t).Preferences()
		restored, err := openSettingsFile(prefs, path, nil)
		if err != nil {
			t.Fatal(err)
		}
		if err := restored.close(); err != nil {
			t.Fatal(err)
		}
		if prefs.BoolWithFallback("showFreeSpace", true) != !free {
			t.Fatal("disk space choice not restored from JSON")
		}
	}
}

func TestGroupedSettingsPrecedenceAndRestart(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	// Nested values override even invalid legacy copies, without losing future fields.
	document := `{"theme":false,"left.brief":"stale","appearance":{"theme":"dark","density":"compact","fontSize":"large","future":true},"views":{"left":{"brief":true,"future":7},"right":{"brief":false},"showHidden":true},"deletion":{"moveToTrash":false},"archives":{"browse":false},"externalApplications":{"revealCommand":"custom-reveal","terminalCommand":"custom-terminal"},"futureRoot":true}`
	if err := os.WriteFile(path, []byte(document), 0o600); err != nil {
		t.Fatal(err)
	}
	for range 2 {
		p := test.NewTempApp(t).Preferences()
		s, err := openSettingsFile(p, path, nil)
		if err != nil {
			t.Fatal(err)
		}
		if err := s.close(); err != nil {
			t.Fatal(err)
		}
		if p.String(prefTheme) != "dark" || p.String(prefDensity) != "compact" || p.String(prefFontSize) != "large" || !p.Bool(prefLeftBrief) || p.Bool(prefRightBrief) || !p.Bool(prefHidden) || p.Bool(prefTrash) || p.Bool(prefBrowseArchives) || p.String(prefRevealCmd) != "custom-reveal" || p.String(prefTerminalCmd) != "custom-terminal" {
			t.Fatal("grouped settings not restored")
		}
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		var values map[string]json.RawMessage
		if err := json.Unmarshal(data, &values); err != nil {
			t.Fatal(err)
		}
		for key := range settingsFields {
			if _, exists := values[key]; exists {
				t.Fatalf("flat key retained: %s", key)
			}
		}
		appearance, err := stateObject(values["appearance"])
		if err != nil {
			t.Fatal(err)
		}
		views, err := stateObject(values["views"])
		if err != nil {
			t.Fatal(err)
		}
		left, err := stateObject(views["left"])
		if err != nil {
			t.Fatal(err)
		}
		if string(appearance["future"]) != "true" || string(left["future"]) != "7" || string(values["futureRoot"]) != "true" {
			t.Fatal("unknown fields lost")
		}
	}
}
