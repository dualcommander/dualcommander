# Project rules
- Any feature/UI/design work: follow skill `dualcommander-guidelines`.
- Any modal dialog or dialog-driven workflow: follow skill `native-modal-dialogs`.
- Every code change in this repo: follow skill `test-every-change` and run `sh scripts/check.sh` until it passes before finishing. Report its result in the final summary.
- Tests must not depend on images or golden/screenshot comparisons. Assert UI structure, state, and behavior instead; image rendering is fragile and OS-dependent.
