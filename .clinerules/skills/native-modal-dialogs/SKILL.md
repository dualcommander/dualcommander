---
name: native-modal-dialogs
description: Required for creating, changing, or reviewing any DualCommander modal dialog or dialog-driven workflow. Enforces OS-native window decorations and equivalent Escape, native-close, and Cancel workflow cancellation.
---

# Native modal dialogs

Always also follow `dualcommander-guidelines` and `test-every-change`.

## Window and input
- Every modal dialog must use a separate OS-decorated window with the native title bar and close button. No borderless/splash windows, drawn close buttons, or in-canvas dialogs as substitutes.
- Reuse `ui.dialogWindow`, `escapeWindow`, `newEscEntry`, and `windowButton` from existing commander dialog helpers. Extend shared helpers only when needed; do not duplicate dialog infrastructure.
- Keep the parent blocked while the dialog is open, including pane keys, mouse actions, menus, and shortcuts. An overlay may block parent input, but dialog controls must remain in the decorated window.
- Apply input blocking centrally to every modal workflow, including Settings/About. Windows uses native activation guards; Linux X11 uses transient-parent/modal hints. macOS keeps normal Fyne window activation: do not force native modality with method swizzling, focus interception, sheet/session hacks, or `ignoresMouseEvents`. Parent may activate on macOS, but pane actions and shortcuts must stay blocked. Document unsupported backends such as Wayland explicitly. Never allow clicks to pass through to applications underneath.
- Release blockers on both hide and close, including cached input windows; restore them when shown again. Keep native window identity consistent through wrappers. Dialog controls may update their own settings while parent actions remain blocked.
- Escape must close the dialog regardless of focused control: text/password entry, button, checkbox, list, or other focusable widget. A canvas key handler alone is insufficient in Fyne.

## Close means cancel
- Escape, the native close button, and Cancel must use the same cancellation path. None may submit, confirm, retry, or advance the operation.
- Cancel the current dialog-driven workflow, not only its visible window. Before work starts, return without side effects. If work is running or waiting for input, cancel its context and unblock waiters so it stops promptly.
- Password/retry dialogs must stop retrying on cancellation. Do not reopen the prompt or continue the pending operation.
- Cancellation must be idempotent and safe alongside job completion. Track explicit confirmation separately so closing after successful submission does not cancel confirmed work.
- Preserve existing data; remove only partial output owned by the cancelled operation. Do not claim rollback of already completed changes unless implemented.
- Clear sensitive input, release modal/input blockers, restore menus and pane focus, and suppress error dialogs for expected cancellation. Leave unrelated jobs running.
- Keep widget/window access on the UI thread. Never block the UI thread waiting for a worker.

## Required verification
- Drive real dialog workflows in headless tests, not only helper calls.
- Test Escape from every focusable control and from the canvas with no control focused.
- Test native window close and Cancel independently. Assert no confirmation callback, follow-up prompt, file creation, or workflow continuation occurs.
- For active or input-waiting jobs, assert cancellation reaches the worker, waiters exit, and owned partial output is cleaned up.
- Assert parent actions are blocked while open and input/focus are restored after closing. Keep a successful-confirmation test too.
- Run `sh scripts/check.sh` until it passes after code changes.
- Headless tests cannot prove OS decorations. Report manual checks for native close controls and Escape behavior on macOS, Windows, and Linux; do not claim unperformed checks passed.