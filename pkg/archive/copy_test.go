package archive

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
	"testing/fstest"
)

func TestStageSelectionAndCleanup(t *testing.T) {
	f := fstest.MapFS{
		"docs/readme.txt":  {Data: []byte("selected")},
		"docs/nested/file": {Data: []byte("nested")},
		"other":            {Data: []byte("unselected")},
	}
	dir, paths, err := Stage(context.Background(), f, []string{"docs"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)
	if len(paths) != 1 || paths[0] != filepath.Join(dir, "docs") {
		t.Fatalf("staged paths: %v", paths)
	}
	if body, err := os.ReadFile(filepath.Join(dir, "docs/nested/file")); err != nil || string(body) != "nested" {
		t.Fatalf("nested output: %q, %v", body, err)
	}
	if _, err := os.Stat(filepath.Join(dir, "other")); !os.IsNotExist(err) {
		t.Fatal("unselected entry extracted")
	}
	ctx, cancel := context.WithCancel(context.Background())
	partial, _, err := Stage(ctx, f, []string{"docs"}, func(int64, int64, string) { cancel() })
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation: %v", err)
	}
	if _, err := os.Stat(partial); !os.IsNotExist(err) {
		t.Fatal("partial staging retained")
	}
	for _, name := range []string{"../escape", "/absolute", "docs/../../escape", `docs\escape`, "C:escape", "."} {
		dir, _, err := Stage(context.Background(), f, []string{name}, nil)
		if err == nil {
			os.RemoveAll(dir)
			t.Fatalf("accepted unsafe selection %q", name)
		}
		if _, err := os.Stat(dir); !os.IsNotExist(err) {
			t.Fatalf("unsafe selection retained staging: %q", name)
		}
	}
	for _, mode := range []fs.FileMode{fs.ModeSymlink, fs.ModeNamedPipe, fs.ModeDevice} {
		unsafe := fstest.MapFS{"docs/link": {Mode: mode, Data: []byte("outside")}}
		dir, _, err := Stage(context.Background(), unsafe, []string{"docs"}, nil)
		if err == nil {
			os.RemoveAll(dir)
			t.Fatalf("accepted special entry: %v", mode)
		}
		if _, err := os.Stat(dir); !os.IsNotExist(err) {
			t.Fatal("unsafe entry retained staging")
		}
	}
}
