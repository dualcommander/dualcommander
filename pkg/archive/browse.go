// Package archive provides read-only archive browsing without extracting whole archives.
package archive

import (
	"context"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/mholt/archives"
)

// Root splits a virtual path at the first regular file on disk.
func Root(name string) (string, string) {
	name = filepath.Clean(name)
	for root := name; ; root = filepath.Dir(root) {
		if info, err := os.Stat(root); err == nil {
			if info.Mode().IsRegular() {
				rel, _ := filepath.Rel(root, name)
				return root, filepath.ToSlash(rel)
			}
			return "", ""
		}
		if filepath.Dir(root) == root {
			return "", ""
		}
	}
}

// Supported uses the library's registered formats, including compression-only formats.
func Supported(name string) bool {
	_, _, err := archives.Identify(context.Background(), filepath.Base(name), nil)
	return err == nil
}

// Open returns a virtual filesystem. Ordinary files are not virtual folders.
func Open(ctx context.Context, name string, password ...string) (fs.FS, error) {
	f, err := archives.FileSystem(ctx, name, nil)
	if err != nil {
		return nil, err
	}
	if file, ok := f.(archives.FileFS); ok && file.Compression == nil {
		return nil, fmt.Errorf("unsupported archive: %s", name)
	}
	if archive, ok := f.(*archives.ArchiveFS); ok {
		setPassword(archive, password)
	}
	if _, err := ReadDir(f, "."); err != nil {
		return nil, passwordError(err)
	}
	// Detach only before publishing the filesystem to callers: the indexing
	// job's context ends, but later member reads must remain usable.
	if archive, ok := f.(*archives.ArchiveFS); ok {
		archive.Context = context.Background()
	}
	return f, nil
}

// ReadDir validates names before they become selectable virtual paths.
func ReadDir(f fs.FS, name string) ([]fs.DirEntry, error) {
	entries, err := fs.ReadDir(f, name)
	if err != nil {
		return nil, err
	}
	for _, e := range entries {
		if !fs.ValidPath(e.Name()) || e.Name() == "." || strings.ContainsAny(e.Name(), `/\:`) {
			return nil, fmt.Errorf("unsafe archive entry: %q", e.Name())
		}
	}
	return entries, nil
}

// Materialize copies one regular member to a private temporary directory.
// Caller owns cleanup. Links and device entries are never opened.
func Materialize(ctx context.Context, f fs.FS, name string) (string, error) {
	info, err := fs.Stat(f, name)
	if err != nil {
		return "", err
	}
	if !info.Mode().IsRegular() || !fs.ValidPath(name) || strings.ContainsAny(name, `\:`) {
		return "", fmt.Errorf("not a safe regular archive member: %s", name)
	}
	// ponytail: cap external previews at 256 MiB; add streamed export for larger members.
	const limit = 256 << 20
	if info.Size() > limit {
		return "", fmt.Errorf("archive member exceeds 256 MiB preview limit")
	}
	in, err := f.Open(name)
	if err != nil {
		return "", passwordError(err)
	}
	defer in.Close()
	dir, err := os.MkdirTemp("", "dualcommander-archive-")
	if err != nil {
		return "", err
	}
	dst := filepath.Join(dir, filepath.Base(name))
	err = func() error {
		out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
		if err != nil {
			return err
		}
		_, copyErr := io.Copy(out, io.LimitReader(&contextReader{ctx, in}, limit+1))
		closeErr := out.Close()
		if copyErr != nil {
			return copyErr
		}
		if closeErr != nil {
			return closeErr
		}
		st, err := os.Stat(dst)
		if err == nil && st.Size() > limit {
			err = fmt.Errorf("archive member exceeds 256 MiB preview limit")
		}
		return err
	}()
	if err != nil {
		os.RemoveAll(dir)
		return "", passwordError(err)
	}
	return dst, nil
}

type contextReader struct {
	ctx context.Context
	r   io.Reader
}

func (r *contextReader) Read(p []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	return r.r.Read(p)
}
