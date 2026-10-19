package fsops

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestTrashCmd(t *testing.T) {
	p := `/d/it's "x"; rm -rf ~`
	for _, goos := range []string{"darwin", "windows", "linux"} {
		args, env := TrashCmd(goos, p)
		inArgs := args[len(args)-1] == p
		inEnv := len(env) == 1 && env[0] == "DUALCOMMANDER_TRASH="+p
		if inArgs == inEnv { // exactly one channel, verbatim
			t.Errorf("%s: %q %q", goos, args, env)
		}
		for _, a := range args[:len(args)-1] {
			if strings.Contains(a, "rm -rf") {
				t.Errorf("%s: path spliced into script", goos)
			}
		}
	}
}

func TestCopyMoveDelete(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	src := filepath.Join(root, "src")
	a, b := filepath.Join(root, "a"), filepath.Join(root, "b")
	for _, d := range []string{filepath.Join(src, "sub"), a, b} {
		os.MkdirAll(d, 0o755)
	}
	os.WriteFile(filepath.Join(src, "sub", "f.txt"), []byte("hello"), 0o644)

	if err := Copy(ctx, []string{src}, a, nil); err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(filepath.Join(a, "src", "sub", "f.txt")); string(got) != "hello" {
		t.Fatalf("copy content %q", got)
	}
	if err := Copy(ctx, []string{src}, filepath.Join(src, "sub"), nil); err == nil {
		t.Fatal("expected copy-into-self error")
	}
	if err := Move(ctx, []string{filepath.Join(a, "src")}, b, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(b, "src", "sub", "f.txt")); err != nil {
		t.Fatal(err)
	}
	// single source + non-existing dst = copy as new name
	if err := Copy(ctx, []string{filepath.Join(src, "sub", "f.txt")}, filepath.Join(b, "g.txt"), nil); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(b, "g.txt")); err != nil {
		t.Fatal(err)
	}
	if err := Delete(ctx, []string{filepath.Join(b, "src")}, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(b, "src")); !os.IsNotExist(err) {
		t.Fatal("not deleted")
	}
}
