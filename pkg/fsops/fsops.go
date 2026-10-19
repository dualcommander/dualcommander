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

// Size returns total byte size of the given paths (recursive).
func Size(ctx context.Context, paths []string) (int64, error) {
	var total int64
	for _, p := range paths {
		err := filepath.WalkDir(p, func(_ string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if ctx.Err() != nil {
				return ctx.Err()
			}
			if d.Type().IsRegular() {
				if fi, err := d.Info(); err == nil {
					total += fi.Size()
				}
			}
			return nil
		})
		if err != nil {
			return total, err
		}
	}
	return total, nil
}

type copier struct {
	ctx         context.Context
	done, total int64
	progress    Progress
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
	total, err := Size(ctx, srcs)
	if err != nil {
		return err
	}
	c := &copier{ctx: ctx, total: total, progress: progress}
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
	for _, s := range srcs {
		t := Target(s, dst, len(srcs))
		if err := checkNotInside(s, t); err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(t), 0o755); err != nil {
			return err
		}
		if os.Rename(s, t) == nil {
			continue
		}
		if err := Copy(ctx, []string{s}, t, progress); err != nil {
			return err
		}
		if err := os.RemoveAll(s); err != nil {
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
	fi, err := os.Lstat(src)
	if err != nil {
		return err
	}
	switch {
	case fi.Mode()&os.ModeSymlink != 0:
		target, err := os.Readlink(src)
		if err != nil {
			return err
		}
		_ = os.Remove(dst)
		return os.Symlink(target, dst)
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
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, perm)
	if err != nil {
		return err
	}
	buf := make([]byte, 1<<20)
	for {
		if c.ctx.Err() != nil {
			out.Close()
			os.Remove(dst) // don't leave partial file
			return c.ctx.Err()
		}
		n, rerr := in.Read(buf)
		if n > 0 {
			if _, err := out.Write(buf[:n]); err != nil {
				out.Close()
				return err
			}
			c.done += int64(n)
			if c.progress != nil {
				c.progress(c.done, c.total, src)
			}
		}
		if errors.Is(rerr, io.EOF) {
			break
		}
		if rerr != nil {
			out.Close()
			return rerr
		}
	}
	return out.Close()
}
