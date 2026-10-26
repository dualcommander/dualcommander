package archive

import (
	"bytes"
	"context"
	"encoding/binary"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cavaliergopher/cpio"
)

func TestAdditionalFormats(t *testing.T) {
	for _, ext := range []string{"tar", "bz2", "xz", "lzma", "7z", "cpio", "deb", "rpm", "zipx"} {
		t.Run(ext, func(t *testing.T) {
			name := filepath.Join("testdata", "sample."+ext)
			if !Supported(name) || !Supported(strings.ToUpper(name)) {
				t.Fatal("format not recognized")
			}
			f, err := Open(context.Background(), name)
			if err != nil {
				t.Fatal(err)
			}
			member := "readme.txt"
			if ext == "bz2" || ext == "xz" || ext == "lzma" {
				member = "sample." + ext
			}
			if err := fs.WalkDir(f, ".", func(name string, entry fs.DirEntry, err error) error { return err }); err != nil {
				t.Fatal(err)
			}
			file, err := Materialize(context.Background(), f, member)
			if err != nil {
				t.Fatal(err)
			}
			defer os.RemoveAll(filepath.Dir(file))
			body, err := os.ReadFile(file)
			want := "hello\n"
			if ext == "zipx" {
				want = strings.Repeat(want, 1000)
			}
			if err != nil || string(body) != want {
				t.Fatalf("content: %q, %v", body, err)
			}
		})
	}
}

func TestPackageSafety(t *testing.T) {
	for _, member := range []string{"../escape", "/absolute", `C:\escape`, "docs/../../escape"} {
		t.Run(member, func(t *testing.T) {
			var body bytes.Buffer
			w := cpio.NewWriter(&body)
			if err := w.WriteHeader(&cpio.Header{Name: member, Mode: cpio.TypeReg | 0o644}); err != nil {
				t.Fatal(err)
			}
			if err := w.Close(); err != nil {
				t.Fatal(err)
			}
			name := filepath.Join(t.TempDir(), "unsafe.cpio")
			if err := os.WriteFile(name, body.Bytes(), 0o600); err != nil {
				t.Fatal(err)
			}
			if _, err := Open(context.Background(), name); err == nil {
				t.Fatal("unsafe package member accepted")
			}
		})
	}
	for _, ext := range []string{"cpio", "deb", "rpm", "lzma", "zipx"} {
		t.Run(ext, func(t *testing.T) {
			name := filepath.Join(t.TempDir(), "broken."+ext)
			if err := os.WriteFile(name, []byte("broken"), 0o600); err != nil {
				t.Fatal(err)
			}
			if _, err := Open(context.Background(), name); err == nil {
				t.Fatal("corrupt format accepted")
			}
			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			if _, err := Open(ctx, filepath.Join("testdata", "sample."+ext)); err == nil {
				t.Fatal("cancelled indexing succeeded")
			}
		})
	}
	for _, header := range [][]byte{nil, make([]byte, 9), {9, 4, 5, 0, 0, 0, 0, 0, 0xff}} {
		r := zipLZMA(bytes.NewReader(header))
		var b [1]byte
		if _, err := r.Read(b[:]); err == nil {
			t.Fatal("invalid ZIP LZMA header accepted")
		}
		r.Close()
	}
	header := make([]byte, 13)
	binary.LittleEndian.PutUint32(header[1:5], 1<<30)
	if _, err := (lzmaFormat{}).OpenReader(bytes.NewReader(header)); err == nil {
		t.Fatal("oversized LZMA dictionary accepted")
	}
}
