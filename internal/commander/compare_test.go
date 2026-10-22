package commander

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"
)

func TestComparisonSettingsAndCommands(t *testing.T) {
	root := fixture(t)
	a := test.NewTempApp(t)
	u := newTestUI(t, a, filepath.Join(root, "src"), filepath.Join(root, "dst"))
	settingsPath := filepath.Join(t.TempDir(), "settings.json")
	store, err := openSettingsFile(a.Preferences(), settingsPath, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { store.close() })
	if !u.compareFilesItem.Disabled || !u.compareFoldersItem.Disabled {
		t.Fatal("unconfigured comparison must be disabled")
	}
	u.settings()
	w := windowByTitle(t, a, "Settings")
	fields := map[string]*escEntry{}
	for _, o := range test.LaidOutObjects(w.Content()) {
		if form, ok := o.(*widget.Form); ok {
			for _, item := range form.Items {
				if item.Text == "Compare files" || item.Text == "Compare folder contents" {
					fields[item.Text] = item.Widget.(*escEntry)
				}
			}
		}
	}
	if len(fields) != 2 {
		t.Fatal("comparison settings missing")
	}
	command := `["compare-app","{left}","{right}"]`
	for _, e := range fields {
		e.SetText(command)
		if e.Validate() != nil {
			t.Fatal(e.Validate())
		}
		e.SetText(`["compare-app","{left}"]`)
		if e.Validate() == nil {
			t.Fatal("missing right path accepted")
		}
		e.SetText(command)
	}
	if u.compareFilesItem.Disabled || u.compareFoldersItem.Disabled {
		t.Fatal("configured comparison must enable menus immediately")
	}
	fields["Compare files"].SetText("")
	if !u.compareFilesItem.Disabled || u.compareFoldersItem.Disabled {
		t.Fatal("blank must disable only its action")
	}
	fields["Compare files"].SetText(command)
	w.Close()
	if err := store.close(); err != nil {
		t.Fatal(err)
	}
	b := test.NewTempApp(t)
	restored, err := openSettingsFile(b.Preferences(), settingsPath, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer restored.close()
	for _, key := range []string{prefCompareFilesCmd, prefCompareFoldersCmd} {
		if b.Preferences().String(key) != command {
			t.Fatal("comparison command not persisted")
		}
	}
	// Continue using original app so pane selection remains deterministic.
	// newTrackedUI verifies commands enable menus again after restart.
	u = newTrackedUI(t, a)
	if u.compareFilesItem.Disabled || u.compareFoldersItem.Disabled {
		t.Fatal("settings not restored")
	}
	u.activate(u.right)
	cmd, err := u.comparisonCommand(true)
	if err != nil || !slices.Equal(cmd.Args, []string{"compare-app", u.left.path, u.right.path}) || cmd.Dir != u.left.path {
		t.Fatalf("folder command: %v %v", cmd, err)
	}
	u.right.cd(u.left.path, "song.mp3")
	cursorTo(t, u.left, "readme.md")
	cursorTo(t, u.right, "song.mp3")
	cmd, err = u.comparisonCommand(false)
	if err != nil || !slices.Equal(cmd.Args, []string{"compare-app", filepath.Join(u.left.path, "readme.md"), filepath.Join(u.right.path, "song.mp3")}) {
		t.Fatalf("file command order changed with active pane: %v %v", cmd, err)
	}
	cursorTo(t, u.right, "photos")
	u.compareFilesItem.Action()
	if !hasWindow(a, "Error") {
		t.Fatal("directory accepted as file")
	}
	tapDialogButton(t, u, "OK")
	previousPath := u.right.path
	u.right.path = filepath.Join(u.left.path, "app.zip")
	if _, err := u.comparisonCommand(true); err == nil {
		t.Fatal("archive comparison accepted")
	}
	u.right.path = previousPath
	a.Preferences().SetString(prefCompareFoldersCmd, `["dualcommander-no-such-comparison-tool","{left}","{right}"]`)
	u.compareFoldersItem.Action()
	if !hasWindow(a, "Error") {
		t.Fatal("missing tool must report launch error")
	}
	tapDialogButton(t, u, "OK")
}

func TestComparisonLaunch(t *testing.T) {
	if os.Getenv("DUALCOMMANDER_TEST_COMPARE") == "1" {
		data, _ := json.Marshal(os.Args[len(os.Args)-2:])
		if err := os.WriteFile(os.Getenv("DUALCOMMANDER_COMPARE_LOG"), data, 0o600); err != nil {
			os.Exit(2)
		}
		os.Exit(2) // Exercise non-difference failures after recording invocation.
	}
	root := fixture(t)
	u := newTestUI(t, test.NewTempApp(t), filepath.Join(root, "src"), filepath.Join(root, "dst"))
	log := filepath.Join(t.TempDir(), "arguments.json")
	t.Setenv("DUALCOMMANDER_TEST_COMPARE", "1")
	t.Setenv("DUALCOMMANDER_COMPARE_LOG", log)
	data, err := json.Marshal([]string{os.Args[0], "-test.run=^TestComparisonLaunch$", "--", "{left}", "{right}"})
	if err != nil {
		t.Fatal(err)
	}
	u.app.Preferences().SetString(prefCompareFoldersCmd, string(data))
	u.updateCompareMenu()
	u.compareFoldersItem.Action()
	waitUntil(t, func() bool { return hasWindow(u.app, "Error") })
	assertUIText(t, windowByTitle(t, u.app, "Error").Content(), "exit status 2")
	tapDialogButton(t, u, "OK")
	data, err = os.ReadFile(log)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(got, []string{u.left.path, u.right.path}) {
		t.Fatalf("launched arguments: %v", got)
	}
}
