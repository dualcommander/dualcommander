package archive

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"strings"

	"github.com/blakesmith/ar"
	"github.com/cavaliergopher/cpio"
	"github.com/cavaliergopher/rpm"
	"github.com/klauspost/compress/zip"
	"github.com/mholt/archives"
	"github.com/ulikunitz/xz/lzma"
	encryptedzip "github.com/yeka/zip"
)

func init() {
	archives.RegisterFormat(lzmaFormat{})
	archives.RegisterFormat(packageFormat{".cpio", "07070", extractCPIO})
	archives.RegisterFormat(packageFormat{".rpm", "\xed\xab\xee\xdb", extractRPM})
	archives.RegisterFormat(packageFormat{".deb", "!<arch>\n", extractDEB})
	zip.RegisterDecompressor(14, zipLZMA)
	encryptedzip.RegisterDecompressor(14, zipLZMA)
}

// packageFormat adapts package readers to archives' lazy, read-only filesystem.
type packageFormat struct {
	ext, magic string
	extract    func(context.Context, io.Reader, archives.FileHandler) error
}

func (f packageFormat) Extension() string { return f.ext }
func (f packageFormat) MediaType() string { return "application/octet-stream" }
func (f packageFormat) Match(ctx context.Context, name string, r io.Reader) (archives.MatchResult, error) {
	m := archives.MatchResult{ByName: strings.HasSuffix(strings.ToLower(name), f.ext)}
	if r != nil {
		b := make([]byte, len(f.magic))
		_, err := io.ReadFull(r, b)
		if err != nil && err != io.EOF && err != io.ErrUnexpectedEOF {
			return m, err
		}
		m.ByStream = string(b) == f.magic
	}
	return m, ctx.Err()
}
func (f packageFormat) Extract(ctx context.Context, r io.Reader, handle archives.FileHandler) error {
	return f.extract(ctx, &contextReader{ctx, r}, func(ctx context.Context, entry archives.FileInfo) error {
		// Package payloads commonly prefix relative names with "./".
		for strings.HasPrefix(entry.NameInArchive, "./") {
			entry.NameInArchive = strings.TrimPrefix(entry.NameInArchive, "./")
		}
		name := strings.TrimSuffix(entry.NameInArchive, "/")
		if name == "." || name == "" {
			if entry.IsDir() {
				return nil
			}
		}
		if !fs.ValidPath(name) || strings.ContainsAny(name, `\:`) {
			return fmt.Errorf("unsafe archive entry: %q", entry.NameInArchive)
		}
		entry.NameInArchive = name
		return handle(ctx, entry)
	})
}

func extractCPIO(ctx context.Context, r io.Reader, handle archives.FileHandler) error {
	reader := cpio.NewReader(r)
	for {
		h, err := reader.Next()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		info := h.FileInfo()
		entry := archives.FileInfo{FileInfo: info, NameInArchive: h.Name, Header: h}
		entry.Open = func() (fs.File, error) {
			return &passwordFile{ReadCloser: io.NopCloser(reader), info: info}, nil
		}
		if err := handle(ctx, entry); err != nil {
			if errors.Is(err, fs.SkipAll) {
				return nil
			}
			return err
		}
	}
}

func extractRPM(ctx context.Context, r io.Reader, handle archives.FileHandler) error {
	pkg, err := rpm.Read(r)
	if err != nil {
		return err
	}
	if pkg.PayloadFormat() != "cpio" {
		return fmt.Errorf("unsupported RPM payload: %s", pkg.PayloadFormat())
	}
	var compression archives.Decompressor
	switch pkg.PayloadCompression() {
	case "", "none":
		return extractCPIO(ctx, r, handle)
	case "gzip":
		compression = archives.Gz{}
	case "bzip2":
		compression = archives.Bz2{}
	case "xz":
		compression = archives.Xz{}
	case "lzma":
		compression = lzmaFormat{}
	case "zstd":
		compression = archives.Zstd{}
	default:
		return fmt.Errorf("unsupported RPM compression: %s", pkg.PayloadCompression())
	}
	input, err := compression.OpenReader(r)
	if err != nil {
		return err
	}
	defer input.Close()
	return extractCPIO(ctx, input, handle)
}

func extractDEB(ctx context.Context, r io.Reader, handle archives.FileHandler) error {
	magic := make([]byte, 8)
	if _, err := io.ReadFull(r, magic); err != nil {
		return err
	}
	if string(magic) != "!<arch>\n" {
		return fmt.Errorf("invalid DEB header")
	}
	reader := ar.NewReader(io.MultiReader(bytes.NewReader(magic), r))
	for {
		h, err := reader.Next()
		if err != nil {
			if err == io.EOF {
				return fmt.Errorf("DEB data archive missing")
			}
			return err
		}
		if h.Size < 0 {
			return fmt.Errorf("invalid DEB member size")
		}
		name := strings.TrimSuffix(h.Name, "/")
		if name != "data.tar" && !strings.HasPrefix(name, "data.tar.") {
			continue
		}
		format, input, err := archives.Identify(ctx, name, reader)
		if err != nil {
			return err
		}
		extractor, ok := format.(archives.Extractor)
		if !ok {
			return fmt.Errorf("unsupported DEB data archive: %s", name)
		}
		return extractor.Extract(ctx, input, handle)
	}
}

type lzmaFormat struct{}

func (lzmaFormat) Extension() string { return ".lzma" }
func (lzmaFormat) MediaType() string { return "application/x-lzma" }
func (lzmaFormat) Match(ctx context.Context, name string, r io.Reader) (archives.MatchResult, error) {
	m := archives.MatchResult{ByName: strings.HasSuffix(strings.ToLower(name), ".lzma")}
	if r != nil {
		b := make([]byte, lzma.HeaderLen)
		_, err := io.ReadFull(r, b)
		if err != nil && err != io.EOF && err != io.ErrUnexpectedEOF {
			return m, err
		}
		m.ByStream = err == nil && lzma.ValidHeader(b)
	}
	return m, ctx.Err()
}
func (lzmaFormat) OpenWriter(w io.Writer) (io.WriteCloser, error) { return lzma.NewWriter(w) }
func (lzmaFormat) OpenReader(r io.Reader) (io.ReadCloser, error) {
	header := make([]byte, lzma.HeaderLen)
	if _, err := io.ReadFull(r, header); err != nil {
		return nil, err
	}
	// ponytail: cap LZMA dictionaries at 64 MiB; raise only with bounded-memory decoding.
	if binary.LittleEndian.Uint32(header[1:5]) > 64<<20 {
		return nil, fmt.Errorf("LZMA dictionary exceeds 64 MiB")
	}
	input, err := lzma.NewReader(io.MultiReader(bytes.NewReader(header), r))
	if err != nil {
		return nil, err
	}
	return io.NopCloser(input), nil
}

// ZIP method 14 stores a version and property length before LZMA properties.
func zipLZMA(r io.Reader) io.ReadCloser {
	header := make([]byte, 9)
	_, err := io.ReadFull(r, header)
	if err == nil && binary.LittleEndian.Uint16(header[2:4]) != 5 {
		err = fmt.Errorf("invalid ZIP LZMA properties")
	}
	if err != nil {
		return io.NopCloser(&failedReader{err})
	}
	input, err := (lzmaFormat{}).OpenReader(io.MultiReader(bytes.NewReader(header[4:]), bytes.NewReader(bytes.Repeat([]byte{0xff}, 8)), r))
	if err != nil {
		return io.NopCloser(&failedReader{err})
	}
	return input
}

type failedReader struct{ err error }

func (r *failedReader) Read([]byte) (int, error) { return 0, r.err }
