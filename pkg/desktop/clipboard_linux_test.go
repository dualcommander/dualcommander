package desktop

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestParseFileURIs(t *testing.T) {
	gnome, err := parseFileURIs("cut\nfile:///tmp/example\n")
	if err != nil || !reflect.DeepEqual(gnome, []string{"/tmp/example"}) {
		t.Fatalf("GNOME file references must import as copies: %v, %v", gnome, err)
	}
	got, err := parseFileURIs("# files\r\nfile:///tmp/a%20b\r\nfile://localhost/tmp/%E6%96%87%E4%BB%B6%0Aname\r\n")
	if err != nil || !reflect.DeepEqual(got, []string{"/tmp/a b", "/tmp/文件\nname"}) {
		t.Fatalf("decoded file paths: %q, %v", got, err)
	}
	for _, input := range []string{"https://example.com/file", "file://remote/tmp/file", "file:relative", "file:///tmp/%00bad", "file:///tmp/%zz", "file:///tmp/a?query", "file:///tmp/a#fragment"} {
		if _, err := parseFileURIs(input); err == nil {
			t.Errorf("accepted untrusted URI %q", input)
		}
	}
}

func TestLinuxNativeClipboard(t *testing.T) {
	if len(clipboardTool(true)) == 0 || len(clipboardTool(false)) == 0 {
		t.Skip("requires native display and wl-copy/wl-paste or xclip")
	}
	path := filepath.Join(t.TempDir(), "space 文件\nname")
	if err := os.WriteFile(path, []byte("content"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := CopyFiles([]string{path}); err != nil {
		t.Fatal(err)
	}
	got, err := ReadFiles(context.Background())
	if err != nil || !reflect.DeepEqual(got, []string{path}) {
		t.Fatalf("native clipboard: %q, %v", got, err)
	}
}
