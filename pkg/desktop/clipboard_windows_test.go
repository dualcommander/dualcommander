package desktop

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestWindowsNativeClipboard(t *testing.T) {
	if os.Getenv("CI") != "" {
		t.Skip("requires interactive Windows clipboard")
	}
	path := filepath.Join(t.TempDir(), "space 文件 ' & $.txt")
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
