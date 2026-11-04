package archive

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
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
	encryptedzip "github.com/yeka/zip"
)

// Pack creates a new archive, retaining selected base names and empty directories.
// Links and special files are rejected; failures remove only this operation's output.
// ZIP supports optional AES-256 encryption of contents, not entry names.
func Pack(ctx context.Context, srcs []string, dst string, progress fsops.Progress, passwords ...string) (err error) {
	if err := ctx.Err(); err != nil {
		return err
	}
	password := ""
	if len(passwords) > 0 {
		password = passwords[0]
	}
	format := ""
	for _, ext := range []string{".tar.gz", ".tar.bz2", ".tar.xz", ".tar.zst", ".tar", ".zip"} {
		if strings.HasSuffix(strings.ToLower(dst), ext) {
			format = ext
			break
		}
	}
	if format == "" || len(srcs) == 0 {
		return fmt.Errorf("choose files and a supported archive extension")
	}
	if password != "" && format != ".zip" {
		return fmt.Errorf("password protection requires ZIP")
	}
	parent, err := filepath.EvalSymlinks(filepath.Dir(dst))
	if err != nil {
		return err
	}
	target, err := filepath.Abs(filepath.Join(parent, filepath.Base(dst)))
	if err != nil {
		return err
	}
	names := make(map[string]bool)
	for _, src := range srcs {
		info, statErr := os.Lstat(src)
		if statErr != nil {
			return statErr
		}
		base := filepath.Base(filepath.Clean(src))
		if !fs.ValidPath(base) || strings.ContainsAny(base, `\:`) || names[base] {
			return fmt.Errorf("duplicate or unsafe archive name: %s", base)
		}
		names[base] = true
		if info.IsDir() {
			real, err := filepath.EvalSymlinks(src)
			if err != nil {
				return err
			}
			real, err = filepath.Abs(real)
			if err != nil {
				return err
			}
			rel, err := filepath.Rel(real, target)
			if err != nil {
				return err
			}
			if rel == "." || filepath.IsLocal(rel) {
				return fmt.Errorf("archive destination must be outside selected directories")
			}
		}
	}
	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	defer func() {
		err = errors.Join(err, out.Close())
		if err != nil {
			err = errors.Join(err, os.Remove(dst))
		}
	}()
	var add func(string, fs.FileInfo) (io.Writer, error)
	var closeArchive func() error
	var compression io.WriteCloser
	if format == ".zip" && password == "" {
		writer := zip.NewWriter(out)
		closeArchive = writer.Close
		add = func(name string, info fs.FileInfo) (io.Writer, error) {
			header, err := zip.FileInfoHeader(info)
			if err != nil {
				return nil, err
			}
			header.Name, header.Method = name, zip.Deflate
			return writer.CreateHeader(header)
		}
	} else if format == ".zip" {
		writer := encryptedzip.NewWriter(out)
		closeArchive = writer.Close
		add = func(name string, info fs.FileInfo) (io.Writer, error) {
			header, err := encryptedzip.FileInfoHeader(info)
			if err != nil {
				return nil, err
			}
			header.Name, header.Method = name, encryptedzip.Deflate
			if !info.IsDir() {
				header.SetPassword(password)
				header.SetEncryptionMethod(encryptedzip.AES256Encryption)
			}
			return writer.CreateHeader(header)
		}
	} else {
		var sink io.Writer = out
		switch format {
		case ".tar.gz":
			compression = gzip.NewWriter(out)
		case ".tar.bz2":
			compression, err = (archives.Bz2{}).OpenWriter(out)
		case ".tar.xz":
			compression, err = (archives.Xz{}).OpenWriter(out)
		case ".tar.zst":
			compression, err = (archives.Zstd{}).OpenWriter(out)
		}
		if err != nil {
			return err
		}
		if compression != nil {
			sink = compression
		}
		writer := tar.NewWriter(sink)
		closeArchive = writer.Close
		add = func(name string, info fs.FileInfo) (io.Writer, error) {
			header, err := tar.FileInfoHeader(info, "")
			if err != nil {
				return nil, err
			}
			header.Name = name
			return writer, writer.WriteHeader(header)
		}
	}
	defer func() {
		err = errors.Join(err, closeArchive())
		if compression != nil {
			err = errors.Join(err, compression.Close())
		}
		if err == nil {
			err = ctx.Err()
		}
	}()
	var done int64
	for _, src := range srcs {
		src = filepath.Clean(src)
		err = filepath.WalkDir(src, func(path string, entry fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if err := ctx.Err(); err != nil {
				return err
			}
			info, err := entry.Info()
			if err != nil {
				return err
			}
			if !info.IsDir() && !info.Mode().IsRegular() {
				return fmt.Errorf("cannot archive link or special file: %s", path)
			}
			rel, err := filepath.Rel(filepath.Dir(src), path)
			if err != nil {
				return err
			}
			name := filepath.ToSlash(rel)
			if !fs.ValidPath(name) || strings.ContainsAny(name, `\:`) {
				return fmt.Errorf("unsafe archive name: %s", name)
			}
			if info.IsDir() {
				name += "/"
			}
			writer, err := add(name, info)
			if err != nil {
				return err
			}
			if progress != nil {
				progress(done, 0, path)
			}
			if info.IsDir() {
				return ctx.Err()
			}
			input, err := os.Open(path)
			if err != nil {
				return err
			}
			_, copyErr := io.Copy(writer, &unpackReader{contextReader{ctx, input}, func(n int) {
				done += int64(n)
				if progress != nil {
					progress(done, 0, path)
				}
			}})
			return errors.Join(copyErr, input.Close())
		})
		if err != nil {
			return err
		}
	}
	return ctx.Err()
}
