package fsops

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestConflictDecisions(t *testing.T) {
	for _, move := range []bool{false, true} {
		for _, action := range []ConflictAction{Skip, SkipAll, Overwrite, OverwriteAll} {
			root := t.TempDir()
			src, dst := filepath.Join(root, "src"), filepath.Join(root, "dst")
			os.MkdirAll(src, 0700)
			os.MkdirAll(dst, 0700)
			var paths []string
			for _, name := range []string{"a", "b"} {
				path := filepath.Join(src, name)
				if err := os.WriteFile(path, []byte("new"), 0600); err != nil { t.Fatal(err) }
				if err := os.WriteFile(filepath.Join(dst, name), []byte("old"), 0600); err != nil { t.Fatal(err) }
				paths = append(paths, path)
			}
			calls := 0
			handler := func(_ context.Context, c Conflict) (ConflictAction, error) {
				calls++
				if c.SourceInfo.Size() != 3 || c.TargetInfo.Size() != 3 || c.SourceInfo.Name() != c.TargetInfo.Name() { t.Fatal("missing conflict metadata") }
				return action, nil
			}
			var err error
			if move { err = MoveWithConflicts(context.Background(), paths, dst, nil, handler) } else { err = CopyWithConflicts(context.Background(), paths, dst, nil, handler) }
			if err != nil { t.Fatal(err) }
			wantCalls := 2
			if action == SkipAll || action == OverwriteAll { wantCalls = 1 }
			if calls != wantCalls { t.Fatalf("decision %v: prompts=%d want=%d", action, calls, wantCalls) }
			for _, path := range paths {
				data, err := os.ReadFile(filepath.Join(dst, filepath.Base(path)))
				want := "old"
				if action == Overwrite || action == OverwriteAll { want = "new" }
				if err != nil || string(data) != want { t.Fatalf("target=%q err=%v", data, err) }
				_, err = os.Stat(path)
				removed := move && want == "new"
				if os.IsNotExist(err) != removed { t.Fatalf("source preservation: move=%v decision=%v err=%v", move, action, err) }
			}
		}
	}
}

func TestCancelledOverwritePreservesTarget(t *testing.T) {
	root := t.TempDir()
	src, dst := filepath.Join(root, "src"), filepath.Join(root, "dst")
	os.WriteFile(src, []byte("replacement"), 0600)
	os.WriteFile(dst, []byte("original"), 0600)
	ctx, cancel := context.WithCancel(context.Background())
	err := CopyWithConflicts(ctx, []string{src}, dst, func(p CopyProgress) { if p.Done > 0 { cancel() } }, func(context.Context, Conflict) (ConflictAction, error) { return Overwrite, nil })
	if !errors.Is(err, context.Canceled) { t.Fatalf("got %v", err) }
	data, err := os.ReadFile(dst)
	if err != nil || string(data) != "original" { t.Fatalf("target lost: %q %v", data, err) }
}