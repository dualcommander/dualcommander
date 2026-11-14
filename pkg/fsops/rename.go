package fsops

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"unicode"
)

type RenameCase int

const (
	RenameKeepCase RenameCase = iota
	RenameLower
	RenameUpper
	RenameTitle
)

// RenameOptions is one pipeline applied to every name in order: find/replace on the whole name,
// then case and extension on the stem, then prefix, counter, and suffix.
type RenameOptions struct {
	Find, Replace  string
	Regex          bool
	Prefix, Suffix string
	Case           RenameCase
	LowerExt       bool
	Counter        bool
	CounterAtEnd   bool
	Start, Step    int
	Digits         int
}

var (
	ErrInvalidName   = errors.New("invalid name")
	ErrDuplicateName = errors.New("duplicate name")
	ErrNameExists    = errors.New("name exists")
)

// RenameNames returns the new name for each input name. It does not touch the file system.
func RenameNames(names []string, o RenameOptions) ([]string, error) {
	var re *regexp.Regexp
	if o.Regex && o.Find != "" {
		var err error
		if re, err = regexp.Compile(o.Find); err != nil {
			return nil, err
		}
	}
	out := make([]string, len(names))
	for i, name := range names {
		switch {
		case re != nil:
			name = re.ReplaceAllString(name, o.Replace)
		case o.Find != "" && !o.Regex:
			name = strings.ReplaceAll(name, o.Find, o.Replace)
		}
		ext := filepath.Ext(name)
		if ext == name { // a dotfile such as ".env" is all stem
			ext = ""
		}
		stem := name[:len(name)-len(ext)]
		switch o.Case {
		case RenameLower:
			stem = strings.ToLower(stem)
		case RenameUpper:
			stem = strings.ToUpper(stem)
		case RenameTitle:
			stem = titleCase(stem)
		}
		if o.LowerExt {
			ext = strings.ToLower(ext)
		}
		if o.Counter {
			n := fmt.Sprintf("%0*d", max(o.Digits, 0), o.Start+i*o.Step)
			if o.CounterAtEnd {
				stem += n
			} else {
				stem = n + stem
			}
		}
		out[i] = o.Prefix + stem + o.Suffix + ext
	}
	return out, nil
}

// titleCase upper-cases the first letter of each word; any non-letter, non-digit starts a new word.
func titleCase(s string) string {
	var b strings.Builder
	start := true
	for _, r := range s {
		if start {
			b.WriteRune(unicode.ToUpper(r))
		} else {
			b.WriteRune(unicode.ToLower(r))
		}
		start = !unicode.IsLetter(r) && !unicode.IsDigit(r)
	}
	return b.String()
}

// CheckRenames returns one error per pair, nil when the pair is valid. taken reports whether a name
// already exists in the target directory. A name vacated by this batch is not treated as taken.
func CheckRenames(oldNames, newNames []string, taken func(string) bool) []error {
	vacated := make(map[string]bool, len(oldNames))
	for _, n := range oldNames {
		vacated[n] = true
	}
	counts := make(map[string]int, len(newNames))
	for _, n := range newNames {
		counts[n]++
	}
	errs := make([]error, len(newNames))
	for i, n := range newNames {
		switch {
		case counts[n] > 1:
			errs[i] = ErrDuplicateName
		case n == oldNames[i]:
			// unchanged name keeps its file, nothing else to check
		case !validName(n):
			errs[i] = ErrInvalidName
		case taken(n) && !vacated[n]:
			errs[i] = ErrNameExists
		}
	}
	return errs
}

// validName rejects names that cannot be created on Linux, macOS, and Windows.
func validName(n string) bool {
	if n == "" || n == "." || n == ".." || strings.TrimSpace(n) == "" {
		return false
	}
	if strings.ContainsAny(n, `/\:*?"<>|`+"\x00") {
		return false
	}
	return !strings.HasSuffix(n, ".") && !strings.HasSuffix(n, " ")
}

// Rename moves dir/oldNames[i] to dir/newNames[i]. Moves go through temporary names so swaps and
// cycles work. On error or cancel, every completed move is undone.
func Rename(ctx context.Context, dir string, oldNames, newNames []string, progress Progress) error {
	if len(oldNames) != len(newNames) {
		return fmt.Errorf("rename: %d old names for %d new names", len(oldNames), len(newNames))
	}
	seen := make(map[string]bool, len(oldNames))
	for _, name := range oldNames {
		if name == "" || name == "." || name == ".." || strings.ContainsAny(name, "/\\\x00") || seen[name] {
			return fmt.Errorf("%w: %q", ErrInvalidName, name)
		}
		seen[name] = true
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	existing := make(map[string]bool, len(entries))
	for _, entry := range entries {
		existing[entry.Name()] = true
	}
	for _, err := range CheckRenames(oldNames, newNames, func(n string) bool {
		return existing[n]
	}) {
		if err != nil {
			return err
		}
	}
	temp, err := os.MkdirTemp(dir, ".dc-rename-")
	if err != nil {
		return err
	}
	// Remove only empty staging directories; failed rollback leaves recoverable files.
	defer os.Remove(temp)
	type move struct{ from, tmp, to string }
	var moves []move
	for i, from := range oldNames {
		if from != newNames[i] {
			moves = append(moves, move{filepath.Join(dir, from), filepath.Join(temp, fmt.Sprint(i)), filepath.Join(dir, newNames[i])})
		}
	}
	// staged: moves[:staged] sit at tmp. placed: moves[:placed] sit at to.
	staged, placed := 0, 0
	// undo reverses only the moves this call made. Names outside the batch are never touched.
	undo := func(cause error) error {
		for j := placed - 1; j >= 0; j-- {
			if err := renameNoReplace(moves[j].to, moves[j].tmp); err != nil {
				cause = errors.Join(cause, fmt.Errorf("rollback staging %s: %w", moves[j].to, err))
			}
		}
		for j := staged - 1; j >= 0; j-- {
			if _, err := os.Lstat(moves[j].from); !os.IsNotExist(err) {
				cause = errors.Join(cause, fmt.Errorf("rollback destination occupied; recover %s", moves[j].tmp))
				continue
			}
			if err := renameNoReplace(moves[j].tmp, moves[j].from); err != nil {
				cause = errors.Join(cause, fmt.Errorf("rollback %s: %w", moves[j].tmp, err))
			}
		}
		return cause
	}
	for staged < len(moves) {
		if err := ctx.Err(); err != nil {
			return undo(err)
		}
		if err := renameNoReplace(moves[staged].from, moves[staged].tmp); err != nil {
			return undo(err)
		}
		staged++
	}
	for placed < len(moves) {
		if err := ctx.Err(); err != nil {
			return undo(err)
		}
		m := moves[placed]
		if _, err := os.Lstat(m.to); !os.IsNotExist(err) {
			return undo(errors.Join(ErrNameExists, err))
		}
		if err := renameNoReplace(m.tmp, m.to); err != nil {
			return undo(err)
		}
		placed++
		if progress != nil {
			progress(int64(placed), int64(len(moves)), filepath.Base(m.to))
		}
	}
	if err := ctx.Err(); err != nil {
		return undo(err)
	}
	return nil
}
