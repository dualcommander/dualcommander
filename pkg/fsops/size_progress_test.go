package fsops

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestSizeWithProgress(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "nested"), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"a", "nested/.hidden"} {
		if err := os.WriteFile(filepath.Join(root, name), []byte("123"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	var previousBytes, previousFiles int64
	var reports int
	bytes, files, err := SizeWithProgress(context.Background(), []string{root}, func(bytes, files int64) {
		if bytes < previousBytes || files < previousFiles {
			t.Fatal("running totals decreased")
		}
		previousBytes, previousFiles = bytes, files
		reports++
	})
	if err != nil || bytes != 6 || files != 2 || previousBytes != bytes || previousFiles != files || reports < 2 {
		t.Fatalf("scan totals: bytes=%d files=%d reports=%d err=%v", bytes, files, reports, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	_, _, err = SizeWithProgress(ctx, []string{root}, func(int64, int64) { cancel() })
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("progress cancellation: %v", err)
	}
}
