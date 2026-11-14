package fsops

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestRenameNames(t *testing.T) {
	for _, tc := range []struct {
		name string
		in   []string
		opts RenameOptions
		want []string
	}{
		{"literal find replace", []string{"IMG_0001.jpg"}, RenameOptions{Find: "IMG_", Replace: "photo_"}, []string{"photo_0001.jpg"}},
		{"literal replacement is not expanded", []string{"a.txt"}, RenameOptions{Find: "a", Replace: "$1"}, []string{"$1.txt"}},
		{"regex groups", []string{"2024-01-02.txt"}, RenameOptions{Find: `(\d{4})-(\d{2})-(\d{2})`, Replace: "${3}.${2}.${1}", Regex: true}, []string{"02.01.2024.txt"}},
		{"empty find keeps name", []string{"abc"}, RenameOptions{Replace: "X"}, []string{"abc"}},
		{"prefix and suffix around stem", []string{"report.pdf"}, RenameOptions{Prefix: "2024_", Suffix: "_final"}, []string{"2024_report_final.pdf"}},
		{"keep case by default", []string{"My File.TXT"}, RenameOptions{}, []string{"My File.TXT"}},
		{"lower stem only", []string{"My File.TXT"}, RenameOptions{Case: RenameLower}, []string{"my file.TXT"}},
		{"lower stem and extension", []string{"My File.TXT"}, RenameOptions{Case: RenameLower, LowerExt: true}, []string{"my file.txt"}},
		{"upper stem", []string{"my file.txt"}, RenameOptions{Case: RenameUpper}, []string{"MY FILE.txt"}},
		{"title words", []string{"my file-name_x.txt"}, RenameOptions{Case: RenameTitle}, []string{"My File-Name_X.txt"}},
		{"counter at end padded", []string{"a", "b", "c"}, RenameOptions{Counter: true, Start: 1, Step: 1, Digits: 3, CounterAtEnd: true}, []string{"a001", "b002", "c003"}},
		{"counter at start with step", []string{"x.jpg", "y.jpg"}, RenameOptions{Counter: true, Start: 5, Step: 10}, []string{"5x.jpg", "15y.jpg"}},
		{"counter follows prefix", []string{"x.jpg"}, RenameOptions{Prefix: "p_", Counter: true, Start: 1, Step: 1, Digits: 2}, []string{"p_01x.jpg"}},
		{"counter precedes suffix", []string{"x.jpg"}, RenameOptions{Suffix: "_s", Counter: true, Start: 7, Step: 1, Digits: 2, CounterAtEnd: true}, []string{"x07_s.jpg"}},
		{"dotfile is all stem", []string{".env"}, RenameOptions{Case: RenameUpper, LowerExt: true}, []string{".ENV"}},
		{"multi dot keeps last extension", []string{"backup.tar.gz"}, RenameOptions{Suffix: "_old"}, []string{"backup.tar_old.gz"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := RenameNames(tc.in, tc.opts)
			if err != nil || !slices.Equal(got, tc.want) {
				t.Fatalf("got %q, %v; want %q", got, err, tc.want)
			}
		})
	}
	if _, err := RenameNames([]string{"a"}, RenameOptions{Find: "(", Regex: true}); err == nil {
		t.Fatal("invalid regexp accepted")
	}
}

func TestCheckRenames(t *testing.T) {
	taken := func(names ...string) func(string) bool {
		return func(n string) bool { return slices.Contains(names, n) }
	}
	for _, bad := range []string{"", " ", ".", "..", "a/b", `a\b`, "a:b", "a*b", "a?b", `a"b`, "a<b", "a>b", "a|b", "a\x00b", "a.", "a "} {
		if errs := CheckRenames([]string{"old.txt"}, []string{bad}, taken()); !errors.Is(errs[0], ErrInvalidName) {
			t.Errorf("%q: got %v, want ErrInvalidName", bad, errs[0])
		}
	}
	if errs := CheckRenames([]string{"bad:name"}, []string{"bad:name"}, taken()); errs[0] != nil {
		t.Errorf("unchanged name flagged: %v", errs[0])
	}
	if errs := CheckRenames([]string{"a.txt", "b.txt"}, []string{"x.txt", "x.txt"}, taken()); !errors.Is(errs[0], ErrDuplicateName) || !errors.Is(errs[1], ErrDuplicateName) {
		t.Errorf("duplicate targets: %v", errs)
	}
	if errs := CheckRenames([]string{"a", "b"}, []string{"b", "b"}, taken("b")); !errors.Is(errs[0], ErrDuplicateName) || !errors.Is(errs[1], ErrDuplicateName) {
		t.Errorf("target equal to unchanged name: %v", errs)
	}
	if errs := CheckRenames([]string{"a"}, []string{"b"}, taken("b")); !errors.Is(errs[0], ErrNameExists) {
		t.Errorf("existing target: %v", errs)
	}
	if errs := CheckRenames([]string{"a", "b"}, []string{"b", "c"}, taken("a", "b")); errs[0] != nil || errs[1] != nil {
		t.Errorf("target vacated by rename flagged: %v", errs)
	}
	if errs := CheckRenames([]string{"readme.md"}, []string{"README.md"}, taken("readme.md")); errs[0] != nil {
		t.Errorf("case-only change flagged: %v", errs[0])
	}
}

func writeRenameFiles(t *testing.T, dir string, files map[string]string) {
	t.Helper()
	for name, body := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func readRenameFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestRenameSwapAndCycle(t *testing.T) {
	dir := t.TempDir()
	writeRenameFiles(t, dir, map[string]string{"a": "A", "b": "B", "c": "C"})
	if err := Rename(context.Background(), dir, []string{"a", "b", "c"}, []string{"b", "c", "a"}, nil); err != nil {
		t.Fatal(err)
	}
	for name, want := range map[string]string{"a": "C", "b": "A", "c": "B"} {
		if got := readRenameFile(t, filepath.Join(dir, name)); got != want {
			t.Errorf("%s = %q, want %q", name, got, want)
		}
	}
}

func TestRenameRefusesExistingTarget(t *testing.T) {
	dir := t.TempDir()
	writeRenameFiles(t, dir, map[string]string{"a": "A", "b": "B"})
	if err := Rename(context.Background(), dir, []string{"a"}, []string{"b"}, nil); !errors.Is(err, ErrNameExists) {
		t.Fatalf("err = %v, want ErrNameExists", err)
	}
	if readRenameFile(t, filepath.Join(dir, "a")) != "A" || readRenameFile(t, filepath.Join(dir, "b")) != "B" {
		t.Fatal("files changed after refused rename")
	}
}

func TestRenameCancelRollsBack(t *testing.T) {
	dir := t.TempDir()
	writeRenameFiles(t, dir, map[string]string{"a": "a", "b": "b", "c": "c"})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	err := Rename(ctx, dir, []string{"a", "b", "c"}, []string{"x", "y", "z"}, func(int64, int64, string) { cancel() })
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled", err)
	}
	for _, name := range []string{"a", "b", "c"} {
		if got := readRenameFile(t, filepath.Join(dir, name)); got != name {
			t.Errorf("%s = %q after cancel, want %q", name, got, name)
		}
	}
	if entries, _ := os.ReadDir(dir); len(entries) != 3 {
		t.Errorf("leftover entries after cancel: %v", entries)
	}
}

func TestRenameNoReplace(t *testing.T) {
	dir := t.TempDir()
	writeRenameFiles(t, dir, map[string]string{"a": "A", "b": "B"})
	if err := renameNoReplace(filepath.Join(dir, "a"), filepath.Join(dir, "b")); err == nil {
		t.Fatal("existing target replaced")
	}
	if readRenameFile(t, filepath.Join(dir, "a")) != "A" || readRenameFile(t, filepath.Join(dir, "b")) != "B" {
		t.Fatal("collision changed files")
	}
}

func TestRenameRejectsUnsafeSources(t *testing.T) {
	for _, sources := range [][]string{{"../outside"}, {"a", "a"}} {
		t.Run(strings.Join(sources, ","), func(t *testing.T) {
			dir := t.TempDir()
			writeRenameFiles(t, dir, map[string]string{"a": "A"})
			targets := make([]string, len(sources))
			for i := range targets {
				targets[i] = fmt.Sprintf("new%d", i)
			}
			if err := Rename(context.Background(), dir, sources, targets, nil); err == nil {
				t.Fatal("unsafe sources accepted")
			}
			if got := readRenameFile(t, filepath.Join(dir, "a")); got != "A" {
				t.Fatal("source changed")
			}
		})
	}
}

func TestRenameFinalProgressCancelRollsBack(t *testing.T) {
	dir := t.TempDir()
	writeRenameFiles(t, dir, map[string]string{"a": "A"})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := Rename(ctx, dir, []string{"a"}, []string{"b"}, func(int64, int64, string) { cancel() }); !errors.Is(err, context.Canceled) {
		t.Fatalf("got %v, want canceled", err)
	}
	if got := readRenameFile(t, filepath.Join(dir, "a")); got != "A" {
		t.Fatal("source changed")
	}
}

func TestRenameCancelChainKeepsFiles(t *testing.T) {
	dir := t.TempDir()
	writeRenameFiles(t, dir, map[string]string{"a": "a", "b": "b"})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	err := Rename(ctx, dir, []string{"a", "b"}, []string{"b", "c"}, func(int64, int64, string) { cancel() })
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled", err)
	}
	if readRenameFile(t, filepath.Join(dir, "a")) != "a" || readRenameFile(t, filepath.Join(dir, "b")) != "b" {
		t.Fatal("chained rename moved files after cancel")
	}
	if _, err := os.Lstat(filepath.Join(dir, "c")); err == nil {
		t.Fatal("c created after cancel")
	}
}

func TestRenameCaseOnlyChange(t *testing.T) {
	dir := t.TempDir()
	writeRenameFiles(t, dir, map[string]string{"readme.md": "text"})
	if err := Rename(context.Background(), dir, []string{"readme.md"}, []string{"README.md"}, nil); err != nil {
		t.Fatal(err)
	}
	if got := readRenameFile(t, filepath.Join(dir, "README.md")); got != "text" {
		t.Fatalf("content = %q, want text", got)
	}
}

func TestRenameCaseOnlyKeepsOtherFile(t *testing.T) {
	dir := t.TempDir()
	writeRenameFiles(t, dir, map[string]string{"readme.md": "lower", "README.md": "upper"})
	lower, err := os.Stat(filepath.Join(dir, "readme.md"))
	if err != nil {
		t.Fatal(err)
	}
	upper, err := os.Stat(filepath.Join(dir, "README.md"))
	if err != nil {
		t.Fatal(err)
	}
	if os.SameFile(lower, upper) {
		t.Skip("case-insensitive file system: both names are one file")
	}
	if err := Rename(context.Background(), dir, []string{"readme.md"}, []string{"README.md"}, nil); !errors.Is(err, ErrNameExists) {
		t.Fatalf("err = %v, want ErrNameExists", err)
	}
	if readRenameFile(t, filepath.Join(dir, "readme.md")) != "lower" || readRenameFile(t, filepath.Join(dir, "README.md")) != "upper" {
		t.Fatal("files changed after refused case-only rename")
	}
}
