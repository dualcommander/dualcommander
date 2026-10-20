# DualCommander

<img src="assets/dualcommander.png" alt="DualCommander icon" width="128">

[Website](https://dualcommander.github.io/) · About DualCommander includes version information and the project website (DualCommander app menu on macOS; Help menu on Windows/Linux).

Cross-platform (macOS / Windows / Linux) two-pane file manager in Go, inspired by
Total Commander / Double Commander. Built on [Fyne](https://fyne.io).

## Download

Download packages from [GitHub Releases](https://github.com/dualcommander/dualcommander/releases):

- macOS: arm64 (Apple Silicon) or amd64 (Intel).
- Windows: amd64.
- Linux: amd64.

Successful [GitHub Actions builds](https://github.com/dualcommander/dualcommander/actions/workflows/build.yml)
also provide downloadable workflow artifacts.

macOS and Windows packages are unsigned and not notarized; OS security prompts
still apply.

For building from source, testing, or packaging, see [CONTRIBUTING.md](CONTRIBUTING.md).

## Keyboard and mouse

Keys (`Mod` = Cmd on macOS, Ctrl on Windows/Linux; all also in native menu bar):
`Tab` switch pane · `Enter` open · `F4` edit in OS text editor · `Shift+F4` create and select new file · `Backspace` up · `Space`/`Insert` mark ·
`Mod+A` / `Mod+Shift+A` mark/unmark all · `F5` copy · `F6` move · `Shift+F6` rename cursor item · `F7`/`Mod+Shift+N` mkdir ·
`F8`/`Del`/`Mod+Backspace` delete (to Trash/Recycle Bin; permanent if turned off in Settings `Mod+,`) · `Mod+1`/`Mod+2` brief/detailed (both views show Name/Size/Date sorting headers; click to sort ascending, again to reverse) · `Mod+Shift+.` hidden files ·
`Mod+Shift+R` reveal in Finder/Explorer · `Mod+U` swap panes · `Mod+E` same folder in other pane ·
`Mod+Up` parent · `F10` quit. Appearance (System/Light/Dark) and Density (Compact/Standard) in View menu; System follows OS live.

Mouse: click = cursor · `Mod+click` toggle mark · `Shift+click` mark range. F5/F6/F8 act on all marked items.
`Ctrl+R` reloads the active pane (`Cmd+R` also works on macOS). Cursor and marks
are preserved. Panes also reload automatically after files are created, written,
renamed, or removed in the shown folder, using native OS notifications through
`fsnotify` (kqueue on macOS, inotify on Linux, ReadDirectoryChangesW on Windows).
Event bursts are debounced; no polling. Archive contents are not watched.

Each pane has a left-side dropdown labelled with its current mount root (for
example `/` or `/Volumes/External`) for switching to a mounted
partition, volume, or drive root. Mounts are read from the OS when opened.
Occupied / total volume space appears as plain text, not a usage bar (not folder
size). Archives show the containing volume. Disk queries run in the background;
unavailable volumes show “Disk usage unavailable”.

No tabs: click `☆` beside the path to favourite the current folder, or `★` to
unstar it. The down arrow opens saved folders; click a folder name to navigate
in that pane, or its `★` to remove it without navigating. Favourites are shared
between panes and remembered across restarts. Only real folders can be starred.

View > Font Size offers seven presets: xx-small (70%), x-small (80%), small (90%),
medium (100%, default), large (115%), x-large (130%), and xx-large (150%).
Text and inline icons resize together;
the choice applies immediately and is remembered across restarts. Density remains
independent. Medium preserves the existing default font size.

On macOS, file icons show Finder-style Quick Look thumbnails when available,
with native file icons as fallback. Only files currently in view request previews;
scrolling loads newly visible files and skips queued files that have left view.
Previews load in the background without changing row size. Archive members and
other platforms keep file-type icons.

## Archives and packages (ZIP, RPM, and others)

Select a supported archive and press `Enter`, or double-click it, to browse its
contents in the current pane like a folder.

| Format | Extensions | In-pane browsing |
| --- | --- | --- |
| ZIP | `.zip` | Yes, read-only |
| TAR | `.tar` | Yes, read-only |
| 7-Zip | `.7z` | Yes, read-only |
| RAR | `.rar` | Yes, read-only |
| gzip | `.gz` | Yes, one decompressed file |
| bzip2 | `.bz2` | Yes, one decompressed file |
| XZ | `.xz` | Yes, one decompressed file |
| Zstandard | `.zst` | Yes, one decompressed file |
| Brotli | `.br` | Yes, one decompressed file |
| LZ4 | `.lz4` | Yes, one decompressed file |
| Lzip | `.lz` | Yes, one decompressed file |
| Snappy (framed) / S2 | `.sz` | Yes, one decompressed file |
| MinLZ | `.mz` | Yes, one decompressed file |
| zlib | `.zz` | Yes, one decompressed file |
| Compressed TAR | `.tar.gz`, `.tar.bz2`, `.tar.xz`, `.tar.zst`, `.tar.br`, `.tar.lz4`, `.tar.lz`, `.tar.sz`, `.tar.mz`, `.tar.zz` | Yes, read-only |
| RPM package | `.rpm` | No; opens with the OS-associated app |
| Debian package | `.deb` | No; opens with the OS-associated app |

Use the extensions listed above for automatic recognition. Legacy bzip (`.bz`)
is not supported; bzip2 (`.bz2`) is. Support here means browsing existing files,
not creating or updating archives.

- Open subfolders with `Enter` or double-click. The path bar includes the archive
  filename followed by the internal folder path.
- Use `Backspace` (clear any typed name search first), `Mod+Up`, or `..` to go up.
  Going up from the archive root returns to its containing folder and selects it.
- Open a regular member with `Enter` or double-click to view a temporary copy in
  the OS-associated app. Copies are limited to 256 MiB per file and removed when
  DualCommander quits. Changes to these copies never update the archive.
- Password-protected ZIP (ZipCrypto/AES), 7z, and RAR archives prompt when a password
  is needed, either when browsing encrypted headers or opening an encrypted member.
  The masked password prompt uses an OS-decorated modal window. Wrong passwords
  prompt again; Cancel, Escape, or closing the window stops the operation.
  Passwords are kept in memory for the current archive only, never saved in preferences.
- Browsing runs in the background with Cancel. Unreadable archives show an error.

Archive browsing is read-only: editing, copying, moving, deleting, creating
folders, Find Files, terminal, and reveal actions are unavailable inside archives.
Nested archives are not browsed in-pane. After restarting, the pane returns to the
outer archive's containing folder rather than reopening its contents.

RPM, DEB, and other installer packages are not supported as virtual folders. They still
open with the OS-associated app, which may launch an installer rather than a viewer.
To inspect those packages without installing, use an external tool's explicit
list/extract action, then browse the extracted folder in DualCommander.

## Terminal and reveal settings

File > Open in Terminal opens the active pane folder. Settings (`Mod+,`) lets you
customize both terminal and reveal commands as JSON argument lists; blank restores
OS defaults. `{path}` expands to the selected item for Reveal; `{dir}` is the active
pane folder. Arguments stay separate; no shell expansion or pipelines.

Defaults: macOS `open -R` / Terminal; Windows Explorer / PowerShell; Linux
`xdg-open` (parent folder) / first installed terminal: `x-terminal-emulator`, `kgx`,
`gnome-terminal`, `konsole`, `xfce4-terminal`, `mate-terminal`, `kitty`, `alacritty`,
or `xterm`. Windows terminal launches get a separate console. Install a terminal
or configure its command if none is available.
Examples for Settings:

```json
["open", "-a", "iTerm", "{dir}"]
["wt.exe", "-d", "{dir}"]
["gnome-terminal", "--working-directory={dir}"]
["dolphin", "--select", "{path}"]
```

## Find files

`Mod+F` / `Alt+F7` Find Files: name pattern and/or text in files under current folder. Uses `rg` (ripgrep) if
installed, else `grep`/`find` (macOS/Linux) or `findstr`/`where` (Windows). Pick a result to jump to it.
Opens in its own window; `Esc` or the title-bar close button closes it and stops the search.

## File operations and saved state

Copy/move/delete run in background with progress + cancel at bottom of window.
Pane paths, view modes, active pane, theme and window size are restored on next start.

## Contributing

Developer setup, checks, project layout, and release instructions live in
[CONTRIBUTING.md](CONTRIBUTING.md).
