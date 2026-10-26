# DualCommander

A keyboard-friendly, two-pane file manager for macOS, Windows, and Linux.
Built with Go and [Fyne](https://fyne.io).

[Website and downloads](https://dualcommander.github.io/) ·
[User manual](https://dualcommander.github.io/manual.html) ·
[Contributing](CONTRIBUTING.md) ·
[MIT License](LICENSE)

## Install

Download packages for your OS and architecture from
[GitHub Releases](https://github.com/dualcommander/dualcommander/releases).
For Linux Snap installation and trust requirements, see [Snap installation](CONTRIBUTING.md#snap-installation).

Choose **Settings → Language**, then restart to change language.

Archives open as read-only folders: ZIP, TAR, GZ, BZ2, XZ, LZMA, 7Z, RAR,
CPIO (newc/CRC), RPM (CPIO payload), and DEB (data payload).
ZIPX supports Store, Deflate, BZip2, LZMA, XZ, and Zstandard; other codecs
report unsupported compression. Archive creation and editing are not supported.

## Development

Use `make run`, `make build`, and `make check`.
See [Contributing](CONTRIBUTING.md) for prerequisites, CI, and releases.
