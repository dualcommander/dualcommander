# Project rules

Shared instructions for all coding agents. Skill sources live in `.agents/skills/`.
Use native skill loading when available; otherwise read the relevant `SKILL.md`
before starting work. Resolve paths below from the repository root.

- Any feature/UI/design work: follow skill `dualcommander-guidelines`.
- Any modal dialog or dialog-driven workflow: follow skill `native-modal-dialogs`.
- Every code change in this repo: follow skill `test-every-change` and run `sh scripts/check.sh` until it passes before finishing. Report its result in the final summary.
- Tests must not depend on images or golden/screenshot comparisons. Assert UI structure, state, and behavior instead; image rendering is fragile and OS-dependent.

## Skills

- [dualcommander-guidelines](.agents/skills/dualcommander-guidelines/SKILL.md): product, UI, and implementation decisions.
- [native-modal-dialogs](.agents/skills/native-modal-dialogs/SKILL.md): modal dialogs and cancellation workflows.
- [test-every-change](.agents/skills/test-every-change/SKILL.md): test-first changes and required validation.
- [fast-git-diff](.agents/skills/fast-git-diff/SKILL.md): Git comparisons and patch review.
- [minimal-readme](.agents/skills/minimal-readme/SKILL.md): README changes and documentation placement.
