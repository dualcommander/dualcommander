package archive

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"strings"

	"github.com/bodgit/sevenzip"
	"github.com/mholt/archives"
	"github.com/nwaples/rardecode/v2"
	zip "github.com/yeka/zip"
)

// ErrPassword requests a password or another attempt; callers must allow cancellation.
var ErrPassword = errors.New("archive password required or incorrect")

func passwordError(err error) error {
	var readErr *sevenzip.ReadError
	if errors.Is(err, rardecode.ErrArchiveEncrypted) || errors.Is(err, rardecode.ErrArchivedFileEncrypted) ||
		errors.Is(err, rardecode.ErrBadPassword) || errors.As(err, &readErr) && readErr.Encrypted {
		return fmt.Errorf("%w: %v", ErrPassword, err)
	}
	return err
}

func setPassword(f *archives.ArchiveFS, passwords []string) {
	password := ""
	if len(passwords) > 0 {
		password = passwords[0]
	}
	switch format := f.Format.(type) {
	case archives.SevenZip:
		format.Password = password
		f.Format = format
	case archives.Rar:
		format.Password = password
		f.Format = passwordRar{format}
	case archives.Zip:
		f.Format = passwordZip{format, password}
	}
}

type passwordRar struct{ archives.Rar }

func (r passwordRar) Extract(ctx context.Context, input io.Reader, handle archives.FileHandler) error {
	err := r.Rar.Extract(ctx, input, func(ctx context.Context, entry archives.FileInfo) error {
		if hdr, ok := entry.Header.(*rardecode.FileHeader); ok && hdr.Encrypted {
			open := entry.Open
			entry.Open = func() (fs.File, error) {
				file, err := open()
				if err != nil {
					return nil, passwordError(err)
				}
				return &passwordFile{ReadCloser: file, info: entry.FileInfo, encrypted: true}, nil
			}
		}
		return handle(ctx, entry)
	})
	if r.Password != "" && err != nil && (errors.Is(err, io.ErrUnexpectedEOF) || errors.Is(err, rardecode.ErrBadHeaderCRC) || errors.Is(err, rardecode.ErrCorruptBlockHeader)) {
		if seeker, ok := input.(io.Seeker); ok {
			if _, seekErr := seeker.Seek(0, io.SeekStart); seekErr != nil {
				return passwordError(err)
			}
			plain := r.Rar
			plain.Password = ""
			probe := plain.Extract(ctx, input, func(context.Context, archives.FileInfo) error { return fs.SkipAll })
			if errors.Is(probe, rardecode.ErrArchiveEncrypted) {
				return fmt.Errorf("%w: %v", ErrPassword, err)
			}
		}
	}
	return passwordError(err)
}

// passwordZip keeps archives' filesystem/indexing and adds encrypted ZIP reading.
type passwordZip struct {
	archives.Zip
	password string
}

func (z passwordZip) Extract(ctx context.Context, input io.Reader, handle archives.FileHandler) error {
	r, ok := input.(archives.ReaderAtSeeker)
	if !ok {
		return fmt.Errorf("ZIP input must support seeking")
	}
	size, err := r.Seek(0, io.SeekEnd)
	if err != nil {
		return err
	}
	reader, err := zip.NewReader(r, size)
	if err != nil {
		return err
	}
	encrypted := false
	for _, file := range reader.File {
		encrypted = encrypted || file.IsEncrypted()
	}
	if !encrypted {
		if _, err := r.Seek(0, io.SeekStart); err != nil {
			return err
		}
		return z.Zip.Extract(ctx, r, handle)
	}
	for _, file := range reader.File {
		if err := ctx.Err(); err != nil {
			return err
		}
		if !fs.ValidPath(strings.TrimSuffix(file.Name, "/")) || strings.ContainsAny(file.Name, `\:`) {
			return fmt.Errorf("unsafe archive entry: %q", file.Name)
		}
		info := file.FileInfo()
		entry := archives.FileInfo{FileInfo: info, NameInArchive: file.Name, Header: file.FileHeader}
		entry.Open = func() (fs.File, error) {
			if file.IsEncrypted() {
				if z.password == "" {
					return nil, ErrPassword
				}
				// ponytail: ZIP decryption buffers compressed data; cap at 256 MiB until reader supports streaming.
				minimum := uint64(12)
				for extra := file.Extra; len(extra) >= 4; {
					size := int(binary.LittleEndian.Uint16(extra[2:4]))
					if size > len(extra)-4 {
						return nil, fmt.Errorf("invalid ZIP extra field")
					}
					if binary.LittleEndian.Uint16(extra[:2]) == 0x9901 {
						if size < 7 || extra[8] < 1 || extra[8] > 3 {
							return nil, fmt.Errorf("invalid ZIP AES header")
						}
						minimum = uint64(4 + 4*extra[8] + 12)
					}
					extra = extra[4+size:]
				}
				if file.CompressedSize64 > 256<<20 || file.CompressedSize64 < minimum {
					return nil, fmt.Errorf("encrypted ZIP member has unsupported compressed size")
				}
				file.SetPassword(z.password)
				file.DeferAuth = true // authenticate at EOF before Materialize publishes the temporary copy
			}
			r, err := file.Open()
			if err != nil {
				if file.IsEncrypted() && !errors.Is(err, zip.ErrAlgorithm) {
					err = fmt.Errorf("%w: %v", ErrPassword, err)
				}
				return nil, err
			}
			return &passwordFile{ReadCloser: r, info: info, encrypted: file.IsEncrypted()}, nil
		}
		err := handle(ctx, entry)
		if errors.Is(err, fs.SkipAll) {
			return nil
		}
		if err != nil {
			return err
		}
	}
	return nil
}

type passwordFile struct {
	io.ReadCloser
	info      fs.FileInfo
	encrypted bool
}

func (f *passwordFile) Stat() (fs.FileInfo, error) { return f.info, nil }

func (f *passwordFile) Read(p []byte) (int, error) {
	n, err := f.ReadCloser.Read(p)
	if err != nil && err != io.EOF && f.encrypted {
		err = fmt.Errorf("%w: %v", ErrPassword, err)
	}
	return n, err
}
