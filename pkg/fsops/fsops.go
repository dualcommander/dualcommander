// Package fsops implements cancellable file operations with progress reporting.
package fsops

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// Trash moves paths to the OS trash / Recycle Bin (restorable via Finder "Put Back" etc.).
// ponytail: one OS process per item (Finder is ~0.3s each); batch into one call if slow for big selections.
func Trash(ctx context.Context, paths []string, progress Progress) error {
	for i, p := range paths {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if progress != nil {
			progress(int64(i), int64(len(paths)), p)
		}
		args, env := TrashCmd(runtime.GOOS, p)
		cmd := exec.CommandContext(ctx, args[0], args[1:]...)
		cmd.Env = append(os.Environ(), env...)
		if out, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("move to trash failed for %s: %v %s", p, err, strings.TrimSpace(string(out)))
		}
	}
	return nil
}

// TrashCmd: native trash command; path passed as argv/env, never spliced into script text.
func TrashCmd(goos, path string) (args, env []string) {
	switch goos {
	case "darwin":
		if _, err := os.Stat("/usr/bin/trash"); err == nil { // macOS 15+: no Finder automation prompt
			return []string{"/usr/bin/trash", path}, nil
		}
		return []string{"osascript", "-e", "on run argv", "-e",
			`tell application "Finder" to delete POSIX file (item 1 of argv)`, "-e", "end run", path}, nil
	case "windows":
		return []string{"powershell", "-NoProfile", "-NonInteractive", "-Command",
				`Add-Type -AssemblyName Microsoft.VisualBasic; $p = $env:DUALCOMMANDER_TRASH; ` +
					`if (Test-Path -LiteralPath $p -PathType Container) { [Microsoft.VisualBasic.FileIO.FileSystem]::DeleteDirectory($p, 'OnlyErrorDialogs', 'SendToRecycleBin') } ` +
					`else { [Microsoft.VisualBasic.FileIO.FileSystem]::DeleteFile($p, 'OnlyErrorDialogs', 'SendToRecycleBin') }`},
			[]string{"DUALCOMMANDER_TRASH=" + path}
	}
	return []string{"gio", "trash", "--", path}, nil // GLib; present on GNOME/KDE/XFCE desktops
}

// Progress receives bytes done / total bytes and current file.
type Progress func(done, total int64, current string)

// CopyProgress reports bytes and successfully copied non-directory entries.
// Symlinks count as files; empty directories do not.
type CopyProgress struct {
	Done, Total           int64
	FilesDone, FilesTotal int64
	Current               string
}

// Size returns total byte size of the given paths (recursive).
func Size(ctx context.Context, paths []string) (int64, error) {
	total, _, err := copySize(ctx, paths)
	return total, err
}

func copySize(ctx context.Context, paths []string) (total, files int64, err error) {
	for _, p := range paths {
		err := filepath.WalkDir(p, func(_ string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if ctx.Err() != nil {
				return ctx.Err()
			}
			if d.Type().IsRegular() {
				fi, err := d.Info()
				if err != nil {
					return err
				}
				total += fi.Size()
			}
			if !d.IsDir() {
				files++
			}
			return nil
		})
		if err != nil {
			return total, files, err
		}
	}
	return total, files, nil
}

type copier struct {
	ctx                   context.Context
	done, total           int64
	progress              Progress
	filesDone, filesTotal int64
	report                func(CopyProgress)
	conflict              ConflictHandler
	all                   ConflictAction
	move                  bool
	skipped               int
}

func (c *copier) notify(src string) {
	if c.report != nil {
		c.report(CopyProgress{c.done, c.total, c.filesDone, c.filesTotal, src})
	}
	if c.progress != nil {
		c.progress(c.done, c.total, src)
	}
}

// Target resolves where src lands for dst: inside dst when dst is an existing
// directory or there are several sources, otherwise dst itself (copy/rename as).
func Target(src, dst string, n int) string {
	if st, err := os.Stat(dst); (err == nil && st.IsDir()) || n > 1 {
		return filepath.Join(dst, filepath.Base(src))
	}
	return dst
}

// Copy copies srcs to dst (see Target) recursively. Existing files are overwritten.
func Copy(ctx context.Context, srcs []string, dst string, progress Progress) error {
	return CopyWithProgress(ctx, srcs, dst, func(p CopyProgress) {
		if progress != nil {
			progress(p.Done, p.Total, p.Current)
		}
	})
}

// CopyWithProgress includes recursive file counts without a second discovery scan.
func CopyWithProgress(ctx context.Context, srcs []string, dst string, progress func(CopyProgress)) error {
	return CopyWithConflicts(ctx, srcs, dst, progress, nil)
}

// CopyWithConflicts asks before replacing existing entries, including nested entries.
func CopyWithConflicts(ctx context.Context, srcs []string, dst string, progress func(CopyProgress), conflict ConflictHandler) error {
	total, files, err := copySize(ctx, srcs)
	if err != nil {
		return err
	}
	c := &copier{ctx: ctx, total: total, filesTotal: files, report: progress, conflict: conflict}
	c.notify("")
	for _, s := range srcs {
		t := Target(s, dst, len(srcs))
		if err := checkNotInside(s, t); err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(t), 0o755); err != nil {
			return err
		}
		if err := c.copyTree(s, t); err != nil {
			return err
		}
	}
	return nil
}

// Move moves srcs to dst (see Target); uses rename, falls back to copy+delete across devices.
func Move(ctx context.Context, srcs []string, dst string, progress Progress) error {
	return MoveWithConflicts(ctx, srcs, dst, progress, nil)
}

// MoveWithConflicts retains skipped source entries, including during directory merges.
func MoveWithConflicts(ctx context.Context, srcs []string, dst string, progress Progress, conflict ConflictHandler) error {
	c := &copier{ctx: ctx, progress: progress, conflict: conflict, move: true}
	for _, s := range srcs {
		t := Target(s, dst, len(srcs))
		if err := checkNotInside(s, t); err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(t), 0o755); err != nil {
			return err
		}
		if err := c.copyTree(s, t); err != nil {
			return err
		}
	}
	return nil
}

// Delete removes paths recursively.
func Delete(ctx context.Context, paths []string, progress Progress) error {
	for i, p := range paths {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if progress != nil {
			progress(int64(i), int64(len(paths)), p)
		}
		if err := os.RemoveAll(p); err != nil {
			return err
		}
	}
	return nil
}

func checkNotInside(src, dst string) error {
	a, _ := filepath.Abs(src)
	b, _ := filepath.Abs(dst)
	if a == b {
		return fmt.Errorf("source and target are the same: %s", a)
	}
	if strings.HasPrefix(b, a+string(filepath.Separator)) {
		return fmt.Errorf("cannot copy %s into itself", a)
	}
	return nil
}

func (c *copier) copyTree(src, dst string) error {
	if err := c.ctx.Err(); err != nil {
		return err
	}
	fi, err := os.Lstat(src)
	if err != nil {
		return err
	}
	ti, err := os.Lstat(dst)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	if ti != nil {
		if os.SameFile(fi, ti) {
			return fmt.Errorf("source and target are the same: %s", src)
		}
		action, err := c.resolve(src, dst, fi, ti)
		if err != nil {
			return err
		}
		if action == Skip {
			c.skipped++
			return nil
		}
		if fi.IsDir() != ti.IsDir() {
			return c.replaceTree(src, dst)
		}
	}
	if c.move && ti == nil && os.Rename(src, dst) == nil {
		return nil
	}
	skipped := c.skipped
	switch {
	case fi.Mode()&os.ModeSymlink != 0:
		target, err := os.Readlink(src)
		if err != nil {
			return err
		}
		tmp, err := os.CreateTemp(filepath.Dir(dst), ".dualcommander-*")
		if err != nil { return err }
		name := tmp.Name()
		tmp.Close()
		os.Remove(name)
		defer os.Remove(name)
		if err := os.Symlink(target, name); err != nil {
			return err
		}
		if err := os.Rename(name, dst); err != nil { return err }
		if c.move { if err := os.Remove(src); err != nil { return err } }
		c.filesDone++
		c.notify(src)
		return nil
	case fi.IsDir():
		if err := os.MkdirAll(dst, fi.Mode().Perm()|0o700); err != nil {
			return err
		}
		entries, err := os.ReadDir(src)
		if err != nil {
			return err
		}
		for _, e := range entries {
			if err := c.copyTree(filepath.Join(src, e.Name()), filepath.Join(dst, e.Name())); err != nil {
				return err
			}
		}
		if c.move && c.skipped == skipped { return os.Remove(src) }
		return nil
	default:
		return c.copyFile(src, dst, fi.Mode().Perm())
	}
}

func (c *copier) copyFile(src, dst string, perm os.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.CreateTemp(filepath.Dir(dst), ".dualcommander-*")
	if err != nil {
		return err
	}
	defer os.Remove(out.Name())
	defer out.Close()
	if err := out.Chmod(perm); err != nil { return err }
	buf := make([]byte, 1<<20)
	for {
		if c.ctx.Err() != nil {
			out.Close()
			return c.ctx.Err()
		}
		n, rerr := in.Read(buf)
		if n > 0 {
			if _, err := out.Write(buf[:n]); err != nil {
				out.Close()
				return err
			}
			c.done += int64(n)
			c.notify(src)
		}
		if errors.Is(rerr, io.EOF) {
			break
		}
		if rerr != nil {
			out.Close()
			return rerr
		}
	}
	if err := out.Close(); err != nil {
		return err
	}
	if err := c.ctx.Err(); err != nil { return err }
	if err := os.Rename(out.Name(), dst); err != nil { return err }
	if c.move { if err := os.Remove(src); err != nil { return err } }
	c.filesDone++
	c.notify(src)
	return nil
}
