package archive

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	zip "github.com/yeka/zip"
)

func TestEncryptedArchives(t *testing.T) {
	t.Setenv("TMPDIR", t.TempDir())
	t.Setenv("TMP", os.Getenv("TMPDIR"))
	t.Setenv("TEMP", os.Getenv("TMPDIR"))
	for _, name := range []string{"password.zip", "password.7z", "password-headers.7z"} {
		t.Run(name, func(t *testing.T) {
			checkEncrypted(t, filepath.Join("testdata", name))
		})
	}
	for _, method := range []zip.EncryptionMethod{zip.AES128Encryption, zip.AES192Encryption, zip.AES256Encryption} {
		name := filepath.Join(t.TempDir(), "aes.zip")
		out, err := os.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		w := zip.NewWriter(out)
		member, err := w.Encrypt("secret.txt", "secret", method)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := member.Write([]byte("secret contents")); err != nil {
			t.Fatal(err)
		}
		if err := w.Close(); err != nil {
			t.Fatal(err)
		}
		if err := out.Close(); err != nil {
			t.Fatal(err)
		}
		checkEncrypted(t, name)
	}
}

func checkEncrypted(t *testing.T, name string) {
	t.Helper()
	for _, password := range []string{"", "wrong", "secret"} {
		ctx, cancel := context.WithCancel(context.Background())
		f, err := Open(ctx, name, password)
		cancel() // published filesystem must outlive indexing job
		var tmp string
		if err == nil {
			tmp, err = Materialize(context.Background(), f, "secret.txt")
		}
		if password != "secret" {
			if !errors.Is(err, ErrPassword) || tmp != "" {
				t.Fatalf("%s password %q: %s, %v", name, password, tmp, err)
			}
			leftovers, _ := filepath.Glob(filepath.Join(os.TempDir(), "dualcommander-archive-*"))
			if len(leftovers) != 0 {
				t.Fatalf("partial plaintext left behind: %v", leftovers)
			}
			continue
		}
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		body, err := os.ReadFile(tmp)
		os.RemoveAll(filepath.Dir(tmp))
		if err != nil || string(body) != "secret contents" {
			t.Fatalf("%s: %q, %v", name, body, err)
		}
	}
}

func TestEncryptedRAR(t *testing.T) {
	for _, name := range []string{"rar5-psw.rar", "rar5-hpsw.rar", "rar3-comment-psw.rar", "rar3-comment-hpsw.rar"} {
		t.Run(name, func(t *testing.T) {
			for _, password := range []string{"", "wrong", "password"} {
				f, err := Open(context.Background(), filepath.Join("testdata", name), password)
				member := "stest1.txt"
				if name[:4] == "rar3" {
					member = "file1.txt"
				}
				var tmp string
				if err == nil {
					tmp, err = Materialize(context.Background(), f, member)
				}
				// This fixture encrypts its comment only; empty members need no password.
				if password != "password" && name != "rar3-comment-psw.rar" {
					if !errors.Is(err, ErrPassword) || tmp != "" {
						t.Fatalf("password %q: %q %v", password, tmp, err)
					}
					continue
				}
				if err != nil {
					t.Fatal(err)
				}
				body, err := os.ReadFile(tmp)
				os.RemoveAll(filepath.Dir(tmp))
				want := 2048
				if name[:4] == "rar3" {
					want = 0
				}
				if err != nil || len(body) != want {
					t.Fatalf("content length %d, %v", len(body), err)
				}
			}
		})
	}
}
