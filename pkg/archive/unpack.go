package archive

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/dualcommander/dualcommander/pkg/fsops"
	"github.com/mholt/archives"
)

// Unpack extracts beside src into a new folder named after the archive stem.
// Existing files, folders, and links are never replaced; collisions add " 1", " 2", etc.
// Failed attempts remove only their own output, including password retries.
func Unpack(ctx context.Context, src, dstDir string, progress fsops.Progress, password ...string) (err error) {
	if err := ctx.Err(); err != nil {
		return err
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	format, input, err := archives.Identify(ctx, filepath.Base(src), in)
	if err != nil {
		return err
	}
	// Strip compound registered extensions; aliases such as .tgz and .zipx use their final suffix.
	base := filepath.Base(src)
	ext := filepath.Ext(base)
	if strings.HasSuffix(strings.ToLower(base), format.Extension()) {
		ext = base[len(base)-len(format.Extension()):]
	}
	stem := strings.TrimSuffix(base, ext)
	if stem == "" {
		stem = base
	}
	dir := filepath.Join(dstDir, stem)
	for n := 1; ; n++ {
		if err := ctx.Err(); err != nil {
			return err
		}
		err := os.Mkdir(dir, 0o700)
		if err == nil {
			break
		}
		if !os.IsExist(err) {
			return err
		}
		dir = filepath.Join(dstDir, fmt.Sprintf("%s %d", stem, n))
	}
	defer func() {
		if err != nil {
			err = errors.Join(err, os.RemoveAll(dir))
		}
	}()
	var done int64
	copyFile := func(name string, reader io.Reader, mode fs.FileMode) error {
		out, err := os.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_EXCL, mode.Perm()|0o600)
		if err != nil {
			return err
		}
		_, copyErr := io.Copy(out, &unpackReader{contextReader{ctx, reader}, func(n int) {
			done += int64(n)
			if progress != nil {
				progress(done, 0, name)
			}
		}})
		return errors.Join(copyErr, out.Close())
	}
	if extractor, ok := format.(archives.Extractor); ok {
		f := &archives.ArchiveFS{Format: extractor}
		setPassword(f, password)
		err = f.Format.Extract(ctx, input, func(ctx context.Context, entry archives.FileInfo) error {
			if err := ctx.Err(); err != nil {
				return err
			}
			name := strings.TrimSuffix(entry.NameInArchive, "/")
			for strings.HasPrefix(name, "./") {
				name = strings.TrimPrefix(name, "./")
			}
			if (name == "." || name == "") && entry.IsDir() {
				return nil
			}
			if !fs.ValidPath(name) || strings.ContainsAny(name, `\:`) || entry.LinkTarget != "" || (!entry.IsDir() && !entry.Mode().IsRegular()) {
				return fmt.Errorf("unsafe archive entry: %q", entry.NameInArchive)
			}
			// reject links/devices rather than recreate them.
			target := filepath.Join(dir, filepath.FromSlash(name))
			if entry.IsDir() {
				return os.MkdirAll(target, 0o700)
			}
			if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
				return err
			}
			file, err := entry.Open()
			if err != nil {
				return err
			}
			return errors.Join(copyFile(target, file, entry.Mode()), file.Close())
		})
	} else if compression, ok := format.(archives.Decompressor); ok {
		reader, openErr := compression.OpenReader(input)
		if openErr != nil {
			return openErr
		}
		err = errors.Join(copyFile(filepath.Join(dir, stem), reader, 0o600), reader.Close())
	} else {
		err = fmt.Errorf("unsupported archive: %s", src)
	}
	if err == nil {
		err = ctx.Err()
	}
	return passwordError(err)
}

type unpackReader struct {
	contextReader
	report func(int)
}

func (r *unpackReader) Read(p []byte) (int, error) {
	n, err := r.contextReader.Read(p)
	r.report(n)
	return n, err
}
