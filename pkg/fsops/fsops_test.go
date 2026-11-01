package fsops

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func TestSameFolderCopy(t *testing.T) {
	for _, tc := range []struct {
		name, stem, ext string
		dir             bool
	}{
		{"report.pdf", "report", ".pdf", false},
		{"README", "README", "", false},
		{".env", ".env", "", false},
		{".config.json", ".config", ".json", false},
		{"backup.tar.gz", "backup", ".tar.gz", false},
		{"backup.TAR.XZ", "backup", ".TAR.XZ", false},
		{"backup.tar.bz2", "backup", ".tar.bz2", false},
		{"backup.tar.zst", "backup", ".tar.zst", false},
		{"report copy.pdf", "report copy", ".pdf", false},
		{"Projects.v1", "Projects.v1", "", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			src := filepath.Join(root, tc.name)
			file := src
			if tc.dir {
				if err := os.Mkdir(src, 0o755); err != nil {
					t.Fatal(err)
				}
				file = filepath.Join(src, "child.txt")
			}
			if err := os.WriteFile(file, []byte("original"), 0o640); err != nil {
				t.Fatal(err)
			}
			for _, suffix := range []string{" copy", " copy 2", " copy 3"} {
				err := CopyWithConflicts(context.Background(), []string{src}, root, nil, func(context.Context, Conflict) (ConflictAction, error) {
					t.Error("duplicate must not ask to overwrite")
					return Skip, nil
				})
				if err != nil {
					t.Fatal(err)
				}
				target := filepath.Join(root, tc.stem+suffix+tc.ext)
				if tc.dir {
					target = filepath.Join(target, "child.txt")
				}
				if data, err := os.ReadFile(target); err != nil || string(data) != "original" {
					t.Fatalf("duplicate %s: %q, %v", target, data, err)
				}
			}
			if data, err := os.ReadFile(file); err != nil || string(data) != "original" {
				t.Fatalf("source changed: %q, %v", data, err)
			}
		})
	}
}

func TestSameFolderCopyCollisions(t *testing.T) {
	root := t.TempDir()
	src := filepath.Join(root, "file.txt")
	if err := os.WriteFile(src, []byte("original"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "file copy.txt"), []byte("keep"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(root, "file copy 2.txt"), 0o755); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(root, "file copy 3.txt")
	if err := os.Symlink("missing", link); err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	for range 4 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := Copy(context.Background(), []string{src}, root, nil); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	for i := 4; i <= 7; i++ {
		if data, err := os.ReadFile(filepath.Join(root, fmt.Sprintf("file copy %d.txt", i))); err != nil || string(data) != "original" {
			t.Fatalf("copy %d: %q, %v", i, data, err)
		}
	}
	if data, err := os.ReadFile(filepath.Join(root, "file copy.txt")); err != nil || string(data) != "keep" {
		t.Fatalf("existing copy changed: %q, %v", data, err)
	}
	if target, err := os.Readlink(link); err != nil || target != "missing" {
		t.Fatalf("existing symlink changed: %q, %v", target, err)
	}
	// First available suffix gets reused, not the highest suffix plus one.
	if err := os.Remove(filepath.Join(root, "file copy 5.txt")); err != nil {
		t.Fatal(err)
	}
	if err := Copy(context.Background(), []string{src}, root, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "file copy 5.txt")); err != nil {
		t.Fatal(err)
	}
}

func TestSameFolderCopyRejectsAncestor(t *testing.T) {
	root := t.TempDir()
	dst := filepath.Join(root, "child")
	if err := os.Mkdir(dst, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "file"), []byte("original"), 0o600); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	// Raw .. path must not bypass the guard; cancellation bounds a broken copy.
	err := Copy(ctx, []string{dst + string(filepath.Separator) + ".."}, dst, func(done, total int64, file string) {
		if done > 0 {
			cancel()
		}
	})
	if err == nil || !strings.Contains(err.Error(), "cannot copy") {
		t.Fatalf("want copy-into-self error, got %v", err)
	}
}

func TestSameFolderCopyCancellation(t *testing.T) {
	root := t.TempDir()
	src := filepath.Join(root, "file")
	if err := os.WriteFile(src, []byte("original"), 0o600); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	err := Copy(ctx, []string{src}, root, func(done, total int64, file string) {
		if done > 0 {
			cancel()
		}
	})
	if err != context.Canceled {
		t.Fatalf("want cancellation, got %v", err)
	}
	entries, err := os.ReadDir(root)
	if err != nil || len(entries) != 1 || entries[0].Name() != "file" {
		t.Fatalf("cancel left partial copy: %v, %v", entries, err)
	}
}

func TestCopyFileCounts(t *testing.T) {
	root := t.TempDir()
	src := filepath.Join(root, "source")
	if err := os.MkdirAll(filepath.Join(src, "empty-directory"), 0o755); err != nil {
		t.Fatal(err)
	}
	for name, body := range map[string]string{"empty": "", "data": "hello"} {
		if err := os.WriteFile(filepath.Join(src, name), []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	var last CopyProgress
	err := CopyWithProgress(context.Background(), []string{src}, filepath.Join(root, "target"), func(p CopyProgress) {
		if p.FilesDone < last.FilesDone || p.FilesDone > p.FilesTotal {
			t.Fatalf("invalid count: %+v", p)
		}
		last = p
	})
	if err != nil {
		t.Fatal(err)
	}
	if last.FilesDone != 2 || last.FilesTotal != 2 || last.Done != 5 || last.Total != 5 {
		t.Fatalf("wrong final progress: %+v", last)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := CopyWithProgress(ctx, []string{src}, filepath.Join(root, "cancelled"), nil); err != context.Canceled {
		t.Fatalf("cancel: %v", err)
	}
}

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
