package archive

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestPackSafetyAndCancellation(t *testing.T) {
	src := t.TempDir()
	file := filepath.Join(src, "file.txt")
	if err := os.WriteFile(file, []byte("contents"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, ext := range []string{".zip", ".tar", ".tar.gz", ".tar.bz2", ".tar.xz", ".tar.zst"} {
		t.Run(ext, func(t *testing.T) {
			dst := filepath.Join(t.TempDir(), "out"+ext)
			ctx, cancel := context.WithCancel(context.Background())
			err := Pack(ctx, []string{src}, dst, func(done, total int64, current string) {
				if done > 0 {
					cancel()
				}
			})
			if !errors.Is(err, context.Canceled) {
				t.Fatalf("cancel: %v", err)
			}
			if _, err := os.Stat(dst); !os.IsNotExist(err) {
				t.Fatalf("partial output retained: %v", err)
			}
			if err := os.WriteFile(dst, []byte("keep"), 0o600); err != nil {
				t.Fatal(err)
			}
			if err := Pack(context.Background(), []string{file}, dst, nil); err == nil {
				t.Fatal("existing destination accepted")
			}
			if body, _ := os.ReadFile(dst); string(body) != "keep" {
				t.Fatal("existing destination modified")
			}
		})
	}
	for _, tc := range []struct {
		srcs          []string
		dst, password string
	}{
		{nil, filepath.Join(t.TempDir(), "out.zip"), ""},
		{[]string{file}, filepath.Join(t.TempDir(), "out.rar"), ""},
		{[]string{file}, filepath.Join(t.TempDir(), "out.tar"), "secret"},
		{[]string{src}, filepath.Join(src, "out.zip"), ""},
		{[]string{file, file}, filepath.Join(t.TempDir(), "out.zip"), ""},
		{[]string{filepath.Join(src, "missing")}, filepath.Join(t.TempDir(), "out.zip"), ""},
	} {
		if err := Pack(context.Background(), tc.srcs, tc.dst, nil, tc.password); err == nil {
			t.Fatalf("invalid request accepted: %v", tc.srcs)
		}
		if _, err := os.Stat(tc.dst); !os.IsNotExist(err) {
			t.Fatalf("failed request retained output: %v", err)
		}
	}
}

func TestPackEncryptedZIP(t *testing.T) {
	src := filepath.Join(t.TempDir(), "secret.txt")
	if err := os.WriteFile(src, []byte("private"), 0o600); err != nil {
		t.Fatal(err)
	}
	dst := filepath.Join(t.TempDir(), "secret.zip")
	if err := Pack(context.Background(), []string{src}, dst, nil, "secret"); err != nil {
		t.Fatal(err)
	}
	for _, password := range []string{"", "wrong", "secret"} {
		out := t.TempDir()
		err := Unpack(context.Background(), dst, out, nil, password)
		if password != "secret" {
			if !errors.Is(err, ErrPassword) {
				t.Fatalf("password %q: %v", password, err)
			}
			if entries, _ := os.ReadDir(out); len(entries) != 0 {
				t.Fatal("failed decrypt retained output")
			}
		} else if err != nil {
			t.Fatal(err)
		}
	}
}

func TestPackRejectsLinks(t *testing.T) {
	src := t.TempDir()
	if err := os.Symlink(t.TempDir(), filepath.Join(src, "link")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	dst := filepath.Join(t.TempDir(), "out.zip")
	if err := Pack(context.Background(), []string{src}, dst, nil); err == nil {
		t.Fatal("symlink accepted")
	}
	if _, err := os.Stat(dst); !os.IsNotExist(err) {
		t.Fatalf("partial output retained: %v", err)
	}
}
