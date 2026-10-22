package commander

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"testing"
	"time"

	"fyne.io/fyne/v2/test"
)

func TestSettingsFileMigrationAndUI(t *testing.T) {
	root := fixture(t)
	a := test.NewTempApp(t)
	u := newTestUI(t, a, filepath.Join(root, "src"), filepath.Join(root, "dst"))
	a.Preferences().SetBool(prefTrash, false)
	a.Preferences().SetStringList(prefFavourites, []string{root})
	path := filepath.Join(t.TempDir(), ".dualcommander", "settings.json")
	want := settingsValues(a.Preferences())
	for key := range want {
		if isStateKey(key) || key == prefFavourites {
			delete(want, key)
		}
	}
	writeErrors := make(chan error, 1)
	s, err := openSettingsFile(a.Preferences(), path, func(err error) {
		select {
		case writeErrors <- err:
		default:
		}
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.close() })
	read := func() map[string]any {
		t.Helper()
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		var values map[string]any
		if err := json.Unmarshal(data, &values); err != nil {
			t.Fatal(err)
		}
		// Flatten only for comparison with the preference API; disk assertions below check grouping.
		var raw map[string]json.RawMessage
		if err := json.Unmarshal(data, &raw); err != nil {
			t.Fatal(err)
		}
		if err := flattenFields(raw, settingsFields); err != nil {
			t.Fatal(err)
		}
		for key, path := range settingsFields {
			delete(values, path[0])
			var value any
			if err := json.Unmarshal(raw[key], &value); err != nil {
				t.Fatal(err)
			}
			values[key] = value
		}
		return values
	}
	// Compare through JSON so list and numeric representations match.
	encoded, err := json.Marshal(want)
	if err != nil {
		t.Fatal(err)
	}
	var migrated map[string]any
	if err := json.Unmarshal(encoded, &migrated); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(read(), migrated) {
		t.Fatal("migration lost preference keys or values")
	}
	if runtime.GOOS != "windows" {
		for file, mode := range map[string]os.FileMode{path: 0o600, filepath.Dir(path): 0o700} {
			info, err := os.Stat(file)
			if err != nil || info.Mode().Perm() != mode {
				t.Fatalf("private settings permissions: %s, %v, %v", file, info, err)
			}
		}
	}
	u.settings()
	w := windowByTitle(t, a, "Settings")
	var changed bool
	for _, object := range test.LaidOutObjects(w.Content()) {
		if choice, ok := object.(*dialogSelect); ok && reflect.DeepEqual(choice.Options, themeModes) {
			choice.SetSelected("dark")
			changed = true
		}
	}
	if !changed {
		t.Fatal("theme control missing")
	}
	// Observe the real background write, not a test-only explicit save.
	deadline := time.NewTimer(10 * time.Second)
	defer deadline.Stop()
	// Filesystem notifications can be coalesced or missed; assert persisted data instead.
	poll := time.NewTicker(10 * time.Millisecond)
	defer poll.Stop()
	for read()[prefTheme] != "dark" {
		select {
		case <-poll.C:
		case err := <-writeErrors:
			t.Fatal(err)
		case <-deadline.C:
			t.Fatal("UI change never reached JSON")
		}
	}
	w.Close()
	u.settings()
	w = windowByTitle(t, a, "Settings")
	for _, object := range test.LaidOutObjects(w.Content()) {
		switch control := object.(type) {
		case *dialogCheck:
			test.Tap(control)
		case *dialogSelect:
			if reflect.DeepEqual(control.Options, densities) {
				control.SetSelected("compact")
			}
			if reflect.DeepEqual(control.Options, fontSizes) {
				control.SetSelected("large")
			}
		case *escEntry:
			control.SetText(`["custom-app","{dir}"]`)
		}
	}
	w.Close()
	for i := 0; i < 20; i++ {
		u.toggleFavourite(u.left.path)
	}
	if err := s.close(); err != nil {
		t.Fatal(err)
	}
	b := test.NewTempApp(t)
	restored, err := openSettingsFile(b.Preferences(), path, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer restored.close()
	v := newTrackedUI(t, b)
	if v.themeMode != "dark" || v.density != "compact" || v.fontSize != "large" || v.left.path != u.left.path || v.right.path != u.right.path || !b.Preferences().Bool(prefTrash) || b.Preferences().Bool(prefBrowseArchives) || b.Preferences().String(prefRevealCmd) != `["custom-app","{dir}"]` || b.Preferences().String(prefTerminalCmd) != `["custom-app","{dir}"]` || b.Preferences().String(prefEditorCmd) != `["custom-app","{dir}"]` || !reflect.DeepEqual(b.Preferences().StringList(prefFavourites), []string{root}) {
		t.Fatal("fresh app did not restore JSON preferences")
	}
}

func TestSettingsFileManualEditsAndValidation(t *testing.T) {
	for _, document := range []string{
		`{`, `null`, `[]`, `{"theme":true}`, `{"trash":"false"}`,
		`{"fontSize":null}`, `{"favourites":[1]}`, `{"win.w":-1}`, `{"win.h":100001}`,
		`{"appearance":null}`, `{"appearance":{"theme":false}}`,
		`{"views":{"left":[]}}`, `{"views":{"showHidden":"yes"}}`,
		`{"deletion":{"moveToTrash":null}}`, `{"archives":false}`,
		`{"externalApplications":{"terminalCommand":12}}`,
	} {
		t.Run(document, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "settings.json")
			if err := os.WriteFile(path, []byte(document), 0o600); err != nil {
				t.Fatal(err)
			}
			p := test.NewTempApp(t).Preferences()
			p.SetString(prefTheme, "light")
			if _, err := openSettingsFile(p, path, nil); err == nil {
				t.Fatal("invalid JSON accepted")
			}
			data, err := os.ReadFile(path)
			if err != nil || string(data) != document || p.String(prefTheme) != "light" {
				t.Fatal("invalid JSON changed preferences or source file")
			}
		})
	}
	p := test.NewTempApp(t).Preferences()
	p.SetBool(prefTrash, false)
	p.SetString(prefTerminalCmd, "stale command")
	path := filepath.Join(t.TempDir(), "settings.json")
	if err := os.WriteFile(path, []byte(`{"theme":"dark","density":"compact","fontSize":"large","favourites":[],"future":{"enabled":true}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	s, err := openSettingsFile(p, path, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !p.BoolWithFallback(prefTrash, true) || p.String(prefTerminalCmd) != "" || p.String(prefTheme) != "dark" || p.String(prefDensity) != "compact" || p.String(prefFontSize) != "large" || len(p.StringList(prefFavourites)) != 0 {
		t.Fatal("manual edits ignored or missing keys retained stale legacy values")
	}
	for i := 0; i < 100; i++ {
		p.SetFloat(prefW, float64(800+i))
	}
	if err := s.close(); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var values map[string]any
	if err := json.Unmarshal(data, &values); err != nil || !reflect.DeepEqual(values["future"], map[string]any{"enabled": true}) {
		t.Fatalf("final write lost latest value or unknown keys: %s, %v", data, err)
	}
	data, err = os.ReadFile(filepath.Join(filepath.Dir(path), "state.json"))
	if err != nil {
		t.Fatal(err)
	}
	values = nil
	if err := json.Unmarshal(data, &values); err != nil || !reflect.DeepEqual(values["window"], map[string]any{"width": float64(899), "height": float64(700)}) {
		t.Fatalf("state lost latest value: %s, %v", data, err)
	}
	files, err := filepath.Glob(filepath.Join(filepath.Dir(path), ".settings-*"))
	if err != nil || len(files) != 0 {
		t.Fatal("temporary settings files left behind")
	}
}

func TestSettingsFileWriteErrors(t *testing.T) {
	p := test.NewTempApp(t).Preferences()
	path := filepath.Join(t.TempDir(), "settings.json")
	errors := make(chan error, 1)
	s, err := openSettingsFile(p, path, func(err error) {
		select {
		case errors <- err:
		default:
		}
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.close() })
	backup := path + ".backup"
	if err := os.Rename(path, backup); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(path, 0o700); err != nil {
		t.Fatal(err)
	}
	p.SetString(prefTheme, "dark")
	select {
	case <-errors:
	case <-time.After(10 * time.Second):
		t.Fatal("background write error not reported")
	}
	if err := s.close(); err == nil {
		t.Fatal("shutdown write error swallowed")
	}
	if !exists(backup) {
		t.Fatal("failed write removed previous data")
	}
	files, err := filepath.Glob(filepath.Join(filepath.Dir(path), ".settings-*"))
	if err != nil || len(files) != 0 {
		t.Fatal("failed write left temporary files")
	}
	if _, err := openSettingsFile(p, filepath.Join(backup, "settings.json"), nil); err == nil {
		t.Fatal("unreadable path accepted")
	}
}

func TestSettingsPath(t *testing.T) {
	home := t.TempDir()
	key := "HOME"
	if runtime.GOOS == "windows" {
		key = "USERPROFILE"
	}
	t.Setenv(key, home)
	path, err := settingsPath()
	if err != nil || path != filepath.Join(home, ".dualcommander", "settings.json") {
		t.Fatalf("settings path: %s, %v", path, err)
	}
}

func TestFavouritesJSON(t *testing.T) {
	root := fixture(t)
	path := filepath.Join(t.TempDir(), ".dualcommander", "settings.json")
	favouritesPath := filepath.Join(filepath.Dir(path), "favourites.json")
	a := test.NewTempApp(t)
	u := newTestUI(t, a, root, root)
	a.Preferences().SetStringList(prefFavourites, []string{root})
	s, err := openSettingsFile(a.Preferences(), path, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer s.close()
	read := func(want []string) {
		t.Helper()
		data, err := os.ReadFile(favouritesPath)
		var paths []string
		if err != nil || json.Unmarshal(data, &paths) != nil || !reflect.DeepEqual(paths, want) {
			t.Fatalf("favourites: %s, %v", data, err)
		}
	}
	read([]string{root})
	test.Tap(u.left.favourite)
	u.toggleFavourite(filepath.Join(root, "src"))
	if err := s.close(); err != nil {
		t.Fatal(err)
	}
	read([]string{filepath.Join(root, "src")})
	if runtime.GOOS != "windows" {
		info, err := os.Stat(favouritesPath)
		if err != nil || info.Mode().Perm() != 0o600 {
			t.Fatal("favourites not private")
		}
	}
	// Companion file wins over settings.json and supports manual edits on restart.
	if err := os.WriteFile(favouritesPath, []byte("[]"), 0o600); err != nil {
		t.Fatal(err)
	}
	b := test.NewTempApp(t)
	restored, err := openSettingsFile(b.Preferences(), path, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(b.Preferences().StringList(prefFavourites)) != 0 {
		t.Fatal("manual favourites edit ignored")
	}
	if err := restored.close(); err != nil {
		t.Fatal(err)
	}
	for _, document := range []string{"null", "{}", "[1]", "["} {
		if err := os.WriteFile(favouritesPath, []byte(document), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := openSettingsFile(b.Preferences(), path, nil); err == nil {
			t.Fatal("invalid favourites accepted")
		}
		data, err := os.ReadFile(favouritesPath)
		if err != nil || string(data) != document {
			t.Fatal("invalid favourites overwritten")
		}
	}
}

func TestSettingsStateSeparation(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.json")
	legacy := `{"theme":"dark","left.path":"/legacy","active":"right","win.w":950,"favourites":["/legacy-star"]}`
	if err := os.WriteFile(path, []byte(legacy), 0o600); err != nil {
		t.Fatal(err)
	}
	p := test.NewTempApp(t).Preferences()
	s, err := openSettingsFile(p, path, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.close(); err != nil {
		t.Fatal(err)
	}
	read := func(name string) map[string]json.RawMessage {
		t.Helper()
		data, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			t.Fatal(err)
		}
		var values map[string]json.RawMessage
		if err := json.Unmarshal(data, &values); err != nil {
			t.Fatal(err)
		}
		return values
	}
	settings, state := read("settings.json"), read("state.json")
	for key := range settingsFields {
		if _, exists := settings[key]; exists {
			t.Fatalf("flat settings key retained: %s", key)
		}
	}
	if err := flattenFields(settings, settingsFields); err != nil {
		t.Fatal(err)
	}
	for key := range stateFields {
		if _, exists := state[key]; exists {
			t.Fatalf("legacy flat state key retained: %s", key)
		}
	}
	if err := flattenState(state); err != nil {
		t.Fatal(err)
	}
	for key := range settingsValues(p) {
		_, inSettings := settings[key]
		_, inState := state[key]
		if inSettings != (!isStateKey(key) && key != prefFavourites) || inState != isStateKey(key) {
			t.Fatalf("wrong file for %s: settings=%v state=%v", key, inSettings, inState)
		}
	}
	if string(state[prefLeft]) != `"/legacy"` || string(state[prefW]) != "950" {
		t.Fatal("legacy state lost")
	}
	// Existing companion files win over stale combined copies, including wrong types.
	if err := os.WriteFile(path, []byte(`{"theme":"light","left.path":false,"favourites":false}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "state.json"), []byte(`{"left.path":"/stale","pane":{"left":{"path":"/current","sort":"name","futurePane":true}},"window":{"width":1234,"height":800,"futureWindow":true},"futureState":true}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "favourites.json"), []byte(`["/current-star"]`), 0o600); err != nil {
		t.Fatal(err)
	}
	q := test.NewTempApp(t).Preferences()
	q.SetString(prefActive, "right")
	restored, err := openSettingsFile(q, path, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := restored.close(); err != nil {
		t.Fatal(err)
	}
	if q.String(prefLeft) != "/current" || q.String(prefTheme) != "light" || q.StringWithFallback(prefActive, "left") != "left" || !reflect.DeepEqual(q.StringList(prefFavourites), []string{"/current-star"}) {
		t.Fatal("companion precedence or defaults incorrect")
	}
	if string(read("state.json")["futureState"]) != "true" {
		t.Fatal("unknown state key lost")
	}
	nested := read("state.json")
	var pane, left, window map[string]json.RawMessage
	if json.Unmarshal(nested["pane"], &pane) != nil || json.Unmarshal(pane["left"], &left) != nil || json.Unmarshal(nested["window"], &window) != nil || string(left["futurePane"]) != "true" || string(window["futureWindow"]) != "true" {
		t.Fatal("nested unknown fields lost")
	}
	b := test.NewTempApp(t).Preferences()
	restarted, err := openSettingsFile(b, path, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := restarted.close(); err != nil {
		t.Fatal(err)
	}
	if b.String(prefLeft) != "/current" || b.String(prefLeftSort) != "name" || b.Float(prefW) != 1234 || b.Float(prefH) != 800 {
		t.Fatal("nested state not restored")
	}
	for _, document := range []string{"{", "null", "[]", `{"left.path":true}`, `{"win.w":-1}`, `{"pane":null}`, `{"pane":{"left":[]}}`, `{"pane":{"active":"length"}}`, `{"pane":{"left":{"path":true}}}`, `{"window":false}`, `{"window":{"height":-1}}`, `{"window":{"width":null}}`} {
		if err := os.WriteFile(filepath.Join(dir, "state.json"), []byte(document), 0o600); err != nil {
			t.Fatal(err)
		}
		before, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := openSettingsFile(q, path, nil); err == nil {
			t.Fatalf("invalid state accepted: %s", document)
		}
		after, err := os.ReadFile(path)
		if err != nil || string(before) != string(after) {
			t.Fatal("invalid state changed settings")
		}
		data, err := os.ReadFile(filepath.Join(dir, "state.json"))
		if err != nil || string(data) != document {
			t.Fatal("invalid state overwritten")
		}
	}
}
