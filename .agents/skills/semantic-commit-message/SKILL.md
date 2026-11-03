---
name: semantic-commit-message
description: After every change task, suggest Conventional Commit messages based on the diff against the latest pushed branch state. Also use when asked for a commit message.
---

# Semantic commit message

- Include suggested commit message(s) in the final response after every task that changes repository files. Suggest only; do not commit, stage, or push unless explicitly requested.
- Follow `fast-git-diff` before reviewing patches.
- Resolve the pushed branch with `git rev-parse --abbrev-ref --symbolic-full-name '@{push}'`; use `@{upstream}` if no push tracking ref resolves. Refresh its remote with `git fetch --no-tags <remote>` once per task before the final comparison. Never assume `origin/main` or compare only with `HEAD`.
- If fetching fails, use the cached remote-tracking ref and explicitly say its freshness could not be verified. If neither tracking ref exists, report that no pushed baseline is known; use `HEAD` for an explicitly labeled uncommitted-only suggestion instead of claiming a pushed comparison. For an unborn branch, review available staged/untracked files and disclose that no commit baseline exists.
- Compare the tracked working tree against the resolved baseline with `git diff <baseline>`, not a three-dot diff or only `--cached`. This includes unpushed commits plus staged and unstaged tracked changes. Discover paths and binary changes with that same baseline, following `fast-git-diff`; review non-ignored untracked text files separately.
- Inspect `git status --short` and `git log --oneline <baseline>..HEAD` for context. Base messages on actual net changes, not only the latest request or existing commit subjects. Do not claim unrelated pre-existing work as work performed in this task.
- Use Conventional Commits: `<type>(<optional scope>): <imperative summary>`. Omit parentheses when no scope helps. Prefer lowercase, concise subjects without a trailing period; aim for 72 characters or fewer.
- Choose types by behavior: `feat` for new user-facing capability, `fix` for a bug, `perf` for measured performance work, `refactor` for behavior-preserving code changes, `docs` for documentation/instructions, `test` for test-only changes, `build` for build/dependency changes, `ci` for CI, and `chore` for other maintenance. Follow established repository scopes where applicable.
- Use `!` and a `BREAKING CHANGE:` body only for a real incompatible change. Add a short body only when needed to explain a constraint or breaking behavior. Do not invent issue IDs, test results, or capabilities.
- Suggest one message for one cohesive change. If the baseline diff contains independent changes, suggest separate messages and identify their file groups; do not stage or split them automatically. If no net changes remain, say so rather than inventing a commit.
- State the comparison ref alongside the suggestion. Mention binary/ignored changes excluded from patch review when present.