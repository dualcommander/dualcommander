---
name: fast-git-diff
description: Keep Git comparisons fast by excluding binary files and ignored paths before generating text patches.
---

# Fast Git diff

- Scope comparisons to relevant files, not whole directory trees.
- List changed paths first with `git diff --name-only -z`. Use the same revision range and `--cached` option for discovery and review.
- Exclude ignored paths before comparing contents. Git normally excludes untracked ignored files, but tracked files can still match ignore rules: check candidates with `git check-ignore --no-index --stdin -z`.
- Identify binary changes with `git diff --numstat -z --no-ext-diff --no-textconv`; entries with `-` counts are binary. Exclude those paths from patch generation. Do not rely on filename extensions alone.
- Generate patches only for remaining text paths with `git --no-pager diff --no-ext-diff --no-textconv --no-color -- <paths>`. Preserve revision/staging options. If no paths remain, skip the command; an empty path list would compare everything.
- Preserve NUL-delimited filenames while filtering; pass paths as separate arguments. Never split filenames on whitespace or newlines.
- Never use `--binary`, `--text`, text conversion, external diff tools, or recursive `--no-index` directory comparisons for routine review.
- Review untracked source files separately using `git ls-files --others --exclude-standard -z`; skip binary contents. Mention omitted binary/ignored changes without dumping their contents.
- Override exclusions only when the user explicitly requests those files.