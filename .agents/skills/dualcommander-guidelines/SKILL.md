---
name: dualcommander-guidelines
description: Overall product and implementation guidance for dualcommander (Go/Fyne two-pane file manager). Use for any feature, UI, or design decision - libraries vs custom code, opening/editing files, archives, fonts/theme, pane tabs and persistence.
---

# dualcommander guidelines

Always also follow skill `test-every-change`.
For any modal dialog or dialog-driven workflow, also follow skill `native-modal-dialogs`.

## Core objective: UI responsiveness
- Treat smooth keyboard repeat, scrolling, pointer feedback, and cancellation as core product requirements, not optional polish. Preserve input order; never drop navigation events to hide slow work.
- Keep UI callbacks bounded. No blocking filesystem/network scans, thumbnail loading, or process waits on the UI thread; use background workers and post widget changes back through the existing UI dispatcher.
- Bound worker concurrency and queued UI updates. Coalesce replaceable progress updates to the latest value; never discard completion, errors, or cancellation. Reject stale results after navigation.
- Reuse virtualized widgets. Avoid directory-wide scans, text measurement, widget allocations, and full-view refreshes on repeated input; stop work once layout limits are known. Refresh only affected visible items where possible.
- Add deterministic regression checks for queue bounds, stale results, and exact navigation state. Use benchmarks/profiles for speed claims; avoid wall-clock timing assertions and image comparisons. Manually check held-arrow navigation and scrolling in large folders.

## 1. Libraries over custom code
- Order: Go stdlib, then a dependency already in `go.mod`, then a small, well-known library, and custom code only last.
- Prefer maintained third-party Go libraries over custom implementations, even for small features when the stdlib does not cover the requirement. Code reduction and lower ownership/maintenance cost are explicit priorities.
- Before adding custom logic, check existing dependencies and maintained libraries. When reviewing code, identify replacements that delete project-owned code; avoid speculative dependencies and unrelated rewrites. Note choices and concrete follow-up candidates in the summary.
- Judge total integration cost, not just dependency count: remove superseded implementations, avoid duplicate validation and unnecessary wrappers, and keep adapters small. Do not trade away security, accessibility, cancellation, progress, error handling, or tested behavior for fewer lines.

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
- Tests: assert theme sizes and layout invariants for each density/size change, without image comparisons. Inspect appearance manually when needed.

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
- Tests: open/close/switch tabs by key, restore after restart (save, then `newUI` on the same app), missing-path fallback, and tab labels/active state. No image comparisons.

## General
- DualCommander does one thing well: file management, not a Swiss army knife. Keep features and menus focused on browsing and managing files; delegate unrelated tasks to external apps.
- Do not expose generic OS service catalogs such as macOS Services. Remove framework-injected Services menus through native integration, without replacing native menus or removing essential About, Settings, Hide, and Quit actions. Match native menu identity, not localized titles, and test removal plus preservation of unrelated items.
- Feel native: native menus, OS shortcut modifier (`KeyModifierShortcutDefault`), the OS's own app to reveal/open files.
- Keep functions short and files few. Note every simplification with a `ponytail:` comment.

## Dialog spacing and progress
- Reuse `ui.dialogContent`: 16px outer margins, dialog theme independent of file-list density. Use 12px gaps between stacked form groups; keep results lists within the same margins.
- Unify dialog and background-operation progress: native Fyne progress widgets, theme colors, thin 3px track using `operationTrackLayout` (or `operationStripLayout` for the clickable background strip). Keep status/count/percentage text outside the track; no custom-painted bars or text inside determinate bars.
- Use determinate progress only when total work is known reliably. Otherwise use `widget.ProgressBarInfinite`; do not pre-scan recursive searches just to manufacture a percentage.
- Start animation only while work runs. Stop and hide on completion, error, cancellation, and window close. Keep widget updates on the UI thread and reject late results after cancellation.
- Test margins, track geometry, and progress lifecycle through widget structure/state, never screenshots. Manually check native appearance and animation.

## Keep configuration and menus simple
- Good defaults beat options. Add a setting only when no default can suit most users, e.g. the editor command.
- Settings dialog: one page, no tabs or sections. Today it holds only "Delete moves to Trash" (pref `trash`, default on). Every new field must be justified in the summary.
- Remember state silently instead of asking: paths, tabs, view mode, window size.
- Menus: only File, Edit, View, Go (plus the OS app/Help menus). No new top-level menus. Submenus go at most one level deep, and only for exclusive choices (Appearance, Density).
- One menu item per action. No duplicate entries for the same command; extra keys like F-keys or `Alt+F7` work without their own menu item.
- Don't add toolbars, context menus or option checkboxes unless the user asks.
- Before adding a setting or a menu item, try to remove or merge one first.
