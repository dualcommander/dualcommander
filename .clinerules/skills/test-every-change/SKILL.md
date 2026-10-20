---
name: test-every-change
description: Mandatory for ANY code change in dualcommander (Go/Fyne file manager) - features, fixes, UI/visual tweaks, refactors. Defines how to test UI workflows and visuals, and requires running scripts/check.sh after every edit.
---

# Test every change

## Rules
1. After every edit to `*.go`, `go.mod`, or `testdata/`: run `sh scripts/check.sh`. Do not continue or report done while it fails.
2. Every new behavior gets a test in the same change. No exceptions for "visual" work.
3. Never weaken or delete a test to make it pass. If the test exposed a real bug, fix the app.

## Where tests go
- `pkg/fsops/fsops_test.go`: file ops, no UI.
- `ui_test.go`: user workflows, driven through `fyne.io/fyne/v2/test` (headless, no window).
- `helpers_test.go`: `fixture`, `newTestUI`, `press`, `cursorTo`, `tapDialogButton`, `waitJobs`, `exists`.

## UI workflow test pattern
```go
root := fixture(t)                                   // deterministic tmp tree
u := newTestUI(t, test.NewTempApp(t), src, dst)      // in-memory prefs
cursorTo(t, u.left, "readme.md")
press(u, fyne.KeyF5)                                 // same path as real key events
tapDialogButton(t, u, "Copy")                        // or type + Enter in focused entry
waitJobs(t, u)                                       // drain background jobs
// assert disk state AND pane state (entries, cursor, focus, marks)
```
- Background work must post UI updates via `u.jobs.do`, never touch widgets from goroutines (`-race` catches it).
- Restart/persistence: call `u.save()`, then `newUI(a)` on the same app.

## UI assertions without images
- Tests must not depend on images: no golden images, screenshot/pixel comparisons, or `test.AssertRendersToImage`. Rendering varies across operating systems, fonts, and drivers.
- Assert widget structure, labels, accessible controls, theme values, layout invariants, focus, and behavior through headless tests instead.
- Replace existing image comparisons with meaningful structural and workflow assertions; preserve coverage rather than skipping failing cases.
- Check visual appearance manually when needed. Never generate or update image baselines.

## Manual-only items
Native menu bar, OS reveal, real-display theme switching are not reachable by the test driver. List them in the final summary for the user to check by hand.
