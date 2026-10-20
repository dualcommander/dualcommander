<p align="center">
  <img src="assets/dualcommander.png" alt="DualCommander icon" width="112">
</p>
<h1 align="center">DualCommander</h1>
<p align="center">
  <strong>Two panes. One file manager.</strong><br>
  For macOS, Windows, and Linux.
</p>
<p align="center">
  <a href="https://dualcommander.github.io/">Website</a> ·
  <a href="https://github.com/dualcommander/dualcommander/releases">Downloads</a> ·
  <a href="CONTRIBUTING.md">Contributing</a>
</p>

Browse folders side by side, manage files with keyboard shortcuts, and explore
archives without extracting them. Built in Go with [Fyne](https://fyne.io).

## Download

Download packages from [GitHub Releases](https://github.com/dualcommander/dualcommander/releases):

- macOS: arm64 (Apple Silicon) or amd64 (Intel), as `.dmg` or `.zip`.
- Windows: amd64.
- Linux: amd64, as `.deb`, `.rpm`, or `.tar.xz`.

Successful [GitHub Actions builds](https://github.com/dualcommander/dualcommander/actions/workflows/build.yml)
also provide downloadable workflow artifacts.

macOS and Windows packages are unsigned and not notarized; OS security prompts
still apply.

For macOS installation, open the DMG and drag DualCommander into Applications.

Linux packages are not self-contained: they use system graphics libraries and
require glibc 2.35 or newer. DEB targets Debian 12+, Ubuntu 22.04+, and Mint 21+;
RPM targets compatible Fedora/RHEL-family releases (not RHEL 9 or older).
These are compatibility targets, not a tested distro matrix. A desktop session
with OpenGL support is required. Install with `sudo apt install ./dualcommander-linux-amd64.deb`
or `sudo dnf install ./dualcommander-linux-amd64.rpm` so dependencies are resolved.

For building from source, testing, or packaging, see [CONTRIBUTING.md](CONTRIBUTING.md).

## Keyboard and mouse

`Mod` = Cmd on macOS, Ctrl on Windows/Linux. Commands are also available in the native menu bar.

| Key | Action |
| --- | --- |
| `Tab` | Switch pane |
| `Enter` | Open |
| `F4` | Edit in the OS text editor |
| `Shift+F4` | Create and select a new file |
| `Backspace` / `Mod+Up` | Go to parent folder |
| `Space` / `Insert` | Mark item |
| `Mod+A` | Mark all |
| `Mod+Shift+A` | Unmark all |
| `F5` | Copy |
| `F6` | Move |
| `F2` / `Shift+F6` | Rename cursor item (`F2` on `..`: select current path) |
| `F7` / `Mod+Shift+N` | Create folder |
| `F8` / `Del` / `Mod+Backspace` | Delete to Trash/Recycle Bin; delete permanently if Trash is turned off in Settings |
| `Mod+,` | Open Settings |
| `Mod+1` | Brief view |
| `Mod+2` | Detailed view |
| `Mod+Shift+.` | Toggle hidden files |
| `Mod+Shift+R` | Reveal in Finder/Explorer |
| `Mod+Shift+O` | Open current folder in Finder/Explorer/file manager |
| `F9` | Open terminal in current folder |
| `Menu` (application key) | Open cursor item's right-click menu |
| `Mod+U` | Swap panes |
| `Mod+E` | Open the same folder in the other pane |
| `Mod+F` / `Alt+F7` | Find files by name or content |
| `Ctrl+R` (also `Cmd+R` on macOS) | Reload active pane, preserving cursor and marks |
| `F10` | Quit |

| Mouse input | Action |
| --- | --- |
| Click | Set cursor |
| Right-click | Open, Open With / Get Info (macOS) or Properties (Windows), reveal, copy path |
| `Mod+click` | Toggle mark |
| `Shift+click` | Mark range |
| Click Name/Size/Date header | Sort ascending; click again to reverse (brief and detailed views) |

`F5` / `F6` / `F8` act on all marked items.

## Archives

Select a supported archive and press `Enter`, or double-click it, to browse its
contents in the current pane like a folder.

Supports ZIP, TAR, 7z, RAR, compressed TAR, and single-file compression formats
including gzip, bzip2, XZ, and Zstandard. Password-protected ZIP, 7z, and RAR are supported.

Archive browsing is **read-only**; nested archives are not browsed in-pane.
Opening a member creates a temporary copy (up to 256 MiB), removed on exit.
**Edits to that copy do not update the archive.**

DEB and RPM packages open with the OS-associated app, which may launch an installer.
Use an external tool to inspect them without installing.
