package commander

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"fyne.io/fyne/v2/test"
)

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
