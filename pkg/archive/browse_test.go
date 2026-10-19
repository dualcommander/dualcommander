package archive

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
	"testing/fstest"
)

func TestBrowseFormats(t *testing.T) {
	for _, ext := range []string{"zip", "tar.gz", "gz"} {
		t.Run(ext, func(t *testing.T) {
			name := filepath.Join(t.TempDir(), "sample."+ext)
			out, err := os.Create(name)
			if err != nil {
				t.Fatal(err)
			}
			member := "docs/readme.txt"
			if ext == "zip" {
				w := zip.NewWriter(out)
				f, err := w.Create(member)
				if err != nil {
					t.Fatal(err)
				}
				if _, err := f.Write([]byte("hello")); err != nil {
					t.Fatal(err)
				}
				if err := w.Close(); err != nil {
					t.Fatal(err)
				}
			} else {
				g := gzip.NewWriter(out)
				if ext == "tar.gz" {
					w := tar.NewWriter(g)
					if err := w.WriteHeader(&tar.Header{Name: member, Mode: 0o644, Size: 5}); err != nil {
						t.Fatal(err)
					}
					if _, err := w.Write([]byte("hello")); err != nil {
						t.Fatal(err)
					}
					if err := w.Close(); err != nil {
						t.Fatal(err)
					}
				} else {
					member = filepath.Base(name)
					if _, err := g.Write([]byte("hello")); err != nil {
						t.Fatal(err)
					}
				}
				if err := g.Close(); err != nil {
					t.Fatal(err)
				}
			}
			if err := out.Close(); err != nil {
				t.Fatal(err)
			}
			if !Supported(name) {
				t.Fatal("format not recognized")
			}
			ctx, cancel := context.WithCancel(context.Background())
			f, err := Open(ctx, name)
			if err != nil {
				t.Fatal(err)
			}
			entries, err := ReadDir(f, ".")
			if err != nil || len(entries) != 1 {
				t.Fatalf("root listing: %v, %v", entries, err)
			}
			cancel()
			if ext != "gz" {
				if entries, err := ReadDir(f, "docs"); err != nil || len(entries) != 1 {
					t.Fatalf("subfolder listing: %v, %v", entries, err)
				}
			}
			file, err := Materialize(context.Background(), f, member)
			if err != nil {
				t.Fatal(err)
			}
			defer os.RemoveAll(filepath.Dir(file))
			body, err := os.ReadFile(file)
			if err != nil || string(body) != "hello" {
				t.Fatalf("member content: %q, %v", body, err)
			}
			root, inner := Root(filepath.Join(name, "docs"))
			if root != name || inner != "docs" {
				t.Fatalf("virtual path: %q, %q", root, inner)
			}
		})
	}
}

func TestRejectUnsafeMembers(t *testing.T) {
	f := fstest.MapFS{
		"link": &fstest.MapFile{Mode: fs.ModeSymlink, Data: []byte("/etc/passwd")},
		"file": &fstest.MapFile{Data: []byte("hello")},
	}
	for _, name := range []string{"link", "../escape", "/absolute", `C:\escape`} {
		if file, err := Materialize(context.Background(), f, name); err == nil {
			os.RemoveAll(filepath.Dir(file))
			t.Fatalf("accepted unsafe member %q", name)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if file, err := Materialize(ctx, f, "file"); err == nil {
		os.RemoveAll(filepath.Dir(file))
		t.Fatal("cancelled extraction succeeded")
	}
	name := filepath.Join(t.TempDir(), "bad.zip")
	if err := os.WriteFile(name, []byte("not a zip"), 0o600); err != nil {
		t.Fatal(err)
	}
	if f, err := Open(context.Background(), name); err == nil {
		if _, err := ReadDir(f, "."); err == nil {
			t.Fatal("corrupt archive accepted")
		}
	}
}
