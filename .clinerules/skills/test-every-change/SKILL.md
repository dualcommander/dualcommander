---
name: test-every-change
description: Test-driven application changes in dualcommander (Go/Fyne file manager). Validate scripts with native tools, not standalone script tests. Run scripts/check.sh for every code change.
---

# Test every change

## Rules
1. After each edit batch to `*.go`, `go.mod`, or `testdata/`: run `sh scripts/check.sh`. Expected failures in the red phase below permit proceeding only to the matching implementation. Unrelated failures must be investigated; never finish with failing checks.
2. Every new application behavior gets a test written or adapted BEFORE implementation. No exceptions for application fixes or "visual" work. Script and CI changes follow the native-validation policy below.
3. Never weaken or delete a test to make it pass. If the test exposed a real bug, fix the app.

## Scripts and CI
- Do not add `scripts/*_test.sh` or `scripts/*_test.py`, mocked command harnesses, or tests that assert workflow text. Do not relocate these tests to another directory or framework.
- Keep Go application tests and the existing development-profile self-check. Standalone packaging, signing, installer, and workflow test scripts are intentionally omitted.
- Validate shell syntax, Python syntax, and workflow syntax with available native tools. Exercise packaging and signing with actual platform tools when available; preserve production validation and error handling.
- Run `sh scripts/check.sh` after changes. Report platform/credential-dependent checks not run; do not claim application checks validate release artifacts.
- Removing explicitly unwanted test scaffolding adds no behavior and needs no replacement test or artificial red phase. Application behavior changes still require the red–green workflow below.

## Required red–green workflow
1. Identify observable behavior and existing tests before changing production code.
2. Red: add or adapt the smallest regression/workflow test for the requested behavior. Run it against unchanged production code and observe the expected failure. Record command and failure reason. Compilation errors, broken fixtures, and unrelated failures do not prove a behavioral regression; use existing APIs where possible.
3. Green: change only enough production code to satisfy that test. Run the focused test again, then `sh scripts/check.sh` until all checks pass.
4. Repeat for each behavior or edge case discovered during implementation. Update tests as requirements evolve, not only after implementation is complete. Keep earlier regression assertions intact.
5. Refactor only with passing tests; rerun affected tests and full checks afterward.
- For already-written code under review, add missing coverage first. A test that already passes adds coverage but is not evidence of a red–green cycle. Do not claim earlier work was test-first retroactively.
- Final summary: report observed red failure, green result, full-check result, and any manual-only checks not performed.

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
