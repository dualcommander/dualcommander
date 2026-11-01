package archive

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestUnpackCancellationAndCollisions(t *testing.T) {
	dst := t.TempDir()
	if err := os.WriteFile(filepath.Join(dst, "sample"), []byte("keep"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(dst, "sample 1"), 0o700); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	err := Unpack(ctx, filepath.Join("testdata", "sample.tar"), dst, func(done, total int64, name string) {
		if done <= 0 || total != 0 || filepath.Dir(name) != filepath.Join(dst, "sample 2") {
			t.Errorf("progress: %d %d %s", done, total, name)
		}
		cancel()
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation: %v", err)
	}
	entries, err := os.ReadDir(dst)
	if err != nil || len(entries) != 2 {
		t.Fatalf("partial output retained: %v %v", entries, err)
	}
	if err := Unpack(context.Background(), filepath.Join("testdata", "sample.tar"), dst, nil); err != nil {
		t.Fatal(err)
	}
	if body, err := os.ReadFile(filepath.Join(dst, "sample")); err != nil || string(body) != "keep" {
		t.Fatalf("existing file changed: %q %v", body, err)
	}
	if body, err := os.ReadFile(filepath.Join(dst, "sample 2", "readme.txt")); err != nil || string(body) != "hello\n" {
		t.Fatalf("collision output: %q %v", body, err)
	}
	if err := Unpack(ctx, filepath.Join("testdata", "sample.tar"), dst, nil); !errors.Is(err, context.Canceled) {
		t.Fatalf("pre-cancelled extraction: %v", err)
	}
}

func TestUnpackEncryptedFormats(t *testing.T) {
	for _, name := range []string{"password.zip", "password.7z", "password-headers.7z", "rar5-psw.rar", "rar5-hpsw.rar", "rar3-comment-psw.rar", "rar3-comment-hpsw.rar"} {
		t.Run(name, func(t *testing.T) {
			dst := t.TempDir()
			for _, password := range []string{"", "wrong", "secret"} {
				if name[:3] == "rar" && password == "secret" {
					password = "password"
				}
				err := Unpack(context.Background(), filepath.Join("testdata", name), dst, nil, password)
				// This existing RAR fixture has empty members readable without a password.
				if (password == "" || password == "wrong") && name != "rar3-comment-psw.rar" {
					if !errors.Is(err, ErrPassword) {
						t.Fatalf("password error: %v", err)
					}
					entries, readErr := os.ReadDir(dst)
					if readErr != nil || len(entries) != 0 {
						t.Fatalf("partial plaintext retained: %v %v", entries, readErr)
					}
				} else if err != nil {
					t.Fatal(err)
				}
			}
		})
	}
}

func TestUnpackDottedName(t *testing.T) {
	dir := t.TempDir()
	body, err := os.ReadFile(filepath.Join("testdata", "sample.tar"))
	if err != nil {
		t.Fatal(err)
	}
	src := filepath.Join(dir, "sample.v1.TAR")
	if err := os.WriteFile(src, body, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := Unpack(context.Background(), src, dir, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "sample.v1", "readme.txt")); err != nil {
		t.Fatal(err)
	}
}
