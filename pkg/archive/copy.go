package archive

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/dualcommander/dualcommander/pkg/fsops"
)

// Stage copies only selected members to a private directory. Caller removes dir.
// Links and special files are rejected; failed attempts remove their own output.
func Stage(ctx context.Context, f fs.FS, names []string, progress fsops.Progress) (dir string, paths []string, err error) {
	dir, err = os.MkdirTemp("", "dualcommander-archive-copy-")
	if err != nil {
		return "", nil, err
	}
	defer func() {
		if err != nil {
			err = passwordError(errors.Join(err, os.RemoveAll(dir)))
		}
	}()
	var done int64
	for _, name := range names {
		if !fs.ValidPath(name) || name == "." || strings.ContainsAny(name, `\:`) {
			return dir, nil, fmt.Errorf("unsafe archive selection: %q", name)
		}
		base := path.Base(name)
		err = fs.WalkDir(f, name, func(member string, entry fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if err := ctx.Err(); err != nil {
				return err
			}
			if !fs.ValidPath(member) || strings.ContainsAny(member, `\:`) || (member != name && !strings.HasPrefix(member, name+"/")) {
				return fmt.Errorf("unsafe archive entry: %q", member)
			}
			info, err := entry.Info()
			if err != nil {
				return err
			}
			target := filepath.Join(dir, base, filepath.FromSlash(strings.TrimPrefix(strings.TrimPrefix(member, name), "/")))
			if info.IsDir() {
				return os.MkdirAll(target, 0o700)
			}
			if !info.Mode().IsRegular() {
				return fmt.Errorf("not a safe regular archive member: %s", member)
			}
			if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
				return err
			}
			in, err := f.Open(member)
			if err != nil {
				return err
			}
			defer in.Close()
			out, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_EXCL, info.Mode().Perm()|0o600)
			if err != nil {
				return err
			}
			_, copyErr := io.Copy(out, &unpackReader{contextReader{ctx, in}, func(n int) {
				done += int64(n)
				if progress != nil {
					progress(done, 0, member)
				}
			}})
			return errors.Join(copyErr, out.Close(), ctx.Err())
		})
		if err != nil {
			return dir, nil, err
		}
		paths = append(paths, filepath.Join(dir, base))
	}
	return dir, paths, ctx.Err()
}
