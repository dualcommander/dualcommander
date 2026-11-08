# Project rules

Shared instructions for all coding agents. Skill sources live in `.agents/skills/`.
Use native skill loading when available; otherwise read the relevant `SKILL.md`
before starting work. Resolve paths below from the repository root.

- Any feature/UI/design work: follow skill `dualcommander-guidelines`.
- Any modal dialog or dialog-driven workflow: follow skill `native-modal-dialogs`.
- Any change affecting privacy disclosures: follow skill `privacy-policy` and update `PRIVACY.md` when behavior changes.
- Any script, Makefile, packaging, signing, publishing, or workflow change that creates files: follow skill `static-files-over-generation`.
- Every code change in this repo: follow skill `test-every-change` and run `make check` until it passes before finishing. Report its result in the final summary.
- After every change task: follow skill `semantic-commit-message` and include suggested Conventional Commit message(s) based on the diff against the latest pushed branch state.
- No visual checks or screenshot generation in tests, including manual or opt-in paths. No canvas captures, pixel inspection, or image/golden comparisons. Assert UI structure, state, and behavior instead; check visual appearance manually outside tests.
- Do not add `ponytail:` markers or speculative TODO comments. Comments should explain current behavior or constraints, not unrequested future work.

## Skills

- [dualcommander-guidelines](.agents/skills/dualcommander-guidelines/SKILL.md): product, UI, and implementation decisions.
- [native-modal-dialogs](.agents/skills/native-modal-dialogs/SKILL.md): modal dialogs and cancellation workflows.
- [test-every-change](.agents/skills/test-every-change/SKILL.md): test-first changes and required validation.
- [static-files-over-generation](.agents/skills/static-files-over-generation/SKILL.md): prefer checked-in static files to heredocs and generated artifacts.
- [fast-git-diff](.agents/skills/fast-git-diff/SKILL.md): Git comparisons and patch review.
- [semantic-commit-message](.agents/skills/semantic-commit-message/SKILL.md): suggest commit messages against the latest pushed changes after every change task.
- [minimal-readme](.agents/skills/minimal-readme/SKILL.md): README changes and documentation placement.
- [privacy-policy](.agents/skills/privacy-policy/SKILL.md): keep privacy disclosures aligned with application behavior.
