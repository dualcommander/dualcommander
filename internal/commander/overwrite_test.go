package commander

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"fyne.io/fyne/v2/test"
	"github.com/dualcommander/dualcommander/pkg/fsops"
)

func TestOverwriteDialog(t *testing.T) {
	for _, choice := range []string{"Skip", "Skip All", "Overwrite", "Overwrite All", "Cancel"} {
		root := fixture(t)
		u := newTestUI(t, test.NewTempApp(t), root, root)
		src := filepath.Join(root, "src", "readme.md")
		info, err := os.Lstat(src)
		if err != nil { t.Fatal(err) }
		type answer struct { action fsops.ConflictAction; err error }
		result := make(chan answer, 1)
		go func() { action, err := u.overwriteConflict(context.Background(), fsops.Conflict{Source: src, Target: src, SourceInfo: info, TargetInfo: info}); result <- answer{action, err} }()
		waitUntil(t, func() bool { return hasWindow(u.app, "Target already exists") })
		w := windowByTitle(t, u.app, "Target already exists")
		assertUIText(t, w.Content(), "Choose what to do with this conflict. All applies only to this operation.", "Skip", "Skip All", "Overwrite", "Overwrite All")
		tapDialogButton(t, u, choice)
		got := <-result
		if choice == "Cancel" { if got.err == nil { t.Fatal("cancel accepted overwrite") }; continue }
		want := map[string]fsops.ConflictAction{"Skip": fsops.Skip, "Skip All": fsops.SkipAll, "Overwrite": fsops.Overwrite, "Overwrite All": fsops.OverwriteAll}[choice]
		if got.err != nil || got.action != want { t.Fatalf("%s: %+v", choice, got) }
	}
}