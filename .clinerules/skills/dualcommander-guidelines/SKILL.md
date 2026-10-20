---
name: dualcommander-guidelines
description: Overall product and implementation guidance for dualcommander (Go/Fyne two-pane file manager). Use for any feature, UI, or design decision - libraries vs custom code, opening/editing files, archives, fonts/theme, pane tabs and persistence.
---

# dualcommander guidelines

Always also follow skill `test-every-change`.
For any modal dialog or dialog-driven workflow, also follow skill `native-modal-dialogs`.

## 1. Libraries over custom code
- Order: Go stdlib, then a dependency already in `go.mod`, then a small, well-known library, and custom code only last.
- Before writing anything over ~30 lines, look for a library that does it. Note the choice in the summary.
- Wrap each library behind one small function in `pkg/` (like `pkg/fsops`), not a big abstraction.

## 2. Edit with OS-native apps
- No F3 viewer/preview (dropped: Enter opens with the default app already). F4 edits in an external app. No built-in editor.
- Defaults by OS:
  - macOS: `open -t <file>` (default text editor).
  - Windows: `notepad <file>`.
  - Linux: `xdg-open <file>`.
- Later, only if asked: a Settings dialog (File > Settings…, `Mod+,`) with an editor command template, e.g. `code -w {file}`, pref key `editorCmd`.
- Run with `exec.Command` and split arguments (`strings.Fields` after replacing `{file}`). Never pass through a shell, which avoids injection from file names. Use `Start()`, never `Wait()` on the UI thread.
- Tests: the command builder is a pure func, so test it per OS template. Also check that the Settings dialog saves its values to prefs.

## 3. Compress / decompress
- Use libraries only:
  - Stdlib `archive/zip`, `archive/tar`, `compress/gzip` for zip/tar/tar.gz.
  - For more formats (7z, rar read, xz, zst), use `github.com/mholt/archives`. Add it only when a format needs it.
- Put the code in `pkg/archive` with `Pack(ctx, srcs, dst, prog)` / `Unpack(ctx, src, dstDir, prog)`.
  - Use the same `fsops.Progress` callback as copy/move and support cancelling via ctx.
  - Remove the partial output if cancelled.
- Security: block zip-slip. Reject entries whose cleaned path escapes `dstDir`, and never follow symlinks out of `dstDir`.
- UI: Alt+F5 pack (dialog with name and format), Alt+F9 unpack to the other pane. Run both as `jobs.run` with progress and Cancel.
- Tests: round-trip pack/unpack, cancel cleanup, and a zip-slip archive that must be rejected.

## 4. Fonts and density like native file managers
- Default text should be smaller than Fyne's 14, matching Finder / Explorer / Nautilus: about 13 on macOS, 12 on Windows, 13 on Linux.
- Also reduce padding and inline icon size so rows are compact, about 22–24px tall in the detailed view.
- Do it in the existing theme wrapper (`variantTheme` in `main.go`) by overriding `Size()`. Don't set sizes on individual widgets.
- No font-size setting. The OS default is the only size.
- Tests: a golden PNG for each density/size change. Inspect the rows visually before accepting.

## 5. Tabs per pane, persistent
- Each pane side holds tabs (`container.DocTabs`), each tab being one folder view.
- Shortcuts:
  - `Mod+T` new tab with the current folder.
  - `Mod+W` close the tab, but never the last one.
  - `Ctrl+Tab` / `Ctrl+Shift+Tab` cycle tabs.
  - Double-clicking a folder with `Mod` held opens it in a new tab.
- The tab title is the folder's base name. Brief/detailed view and hidden-files are per tab.
- Persist per side as JSON in prefs `leftTabs` / `rightTabs`: `{"active":0,"tabs":[{"path":"...","brief":false}]}`.
  - Save on every tab change and in `quit()`.
  - On load, tabs whose path no longer exists fall back to home. Never save an empty tab list.
- Move the old `prefLeft`/`prefRight` values into the new format once.
- Tests: open/close/switch tabs by key, restore after restart (save, then `newUI` on the same app), missing-path fallback, and a golden PNG of the tab bar.

## General
- Feel native: native menus, OS shortcut modifier (`KeyModifierShortcutDefault`), the OS's own app to reveal/open files.
- Keep functions short and files few. Note every simplification with a `ponytail:` comment.

## Keep configuration and menus simple
- Good defaults beat options. Add a setting only when no default can suit most users, e.g. the editor command.
- Settings dialog: one page, no tabs or sections. Today it holds only "Delete moves to Trash" (pref `trash`, default on). Every new field must be justified in the summary.
- Remember state silently instead of asking: paths, tabs, view mode, window size.
- Menus: only File, Edit, View, Go (plus the OS app/Help menus). No new top-level menus. Submenus go at most one level deep, and only for exclusive choices (Appearance, Density).
- One menu item per action. No duplicate entries for the same command; extra keys like F-keys or `Alt+F7` work without their own menu item.
- Don't add toolbars, context menus or option checkboxes unless the user asks.
- Before adding a setting or a menu item, try to remove or merge one first.
