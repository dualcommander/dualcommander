---
name: static-files-over-generation
description: Prefer checked-in static files to heredocs, inline-generated files, or build-time construction of config/manifest/metadata. Use for script, Makefile, packaging, signing, publishing, and workflow changes.
---

# Static files over generation

If content is known at commit time, check it in as a real file. Reviewers can diff and lint files; they cannot do that with text embedded in a script.

- Do not build manifests, plists, entitlements, specs, control files, YAML, JSON, or config with heredocs, `printf`/`echo >`, here-strings, or embedded `python -c` / `python - <<EOF` programs. Commit the file in its native format (under `packaging/`, `snap/`, or beside the app) so native tools and editors can validate it.
- Public, stable identity (package or bundle ID, publisher name, team ID, product name) is not secret. Write it literally in the static file; do not inject it through env vars, repository variables, or script arguments. Env vars are for secrets and per-run inputs.
- Substitute only release-specific values (version, architecture, build number, signing identity) into a staged copy of the static file, using one plain tool (`sed`, `String.Replace`). The checked-in file stays the source of truth.
- A few fixed variants (for example per architecture) get one static file each, not one generated file with conditionals.
- Workflow `run:` steps stay short and never write content of their own; they copy or substitute a checked-in file. Multi-step logic lives in a script called by Make (see `test-every-change`).
- Binary assets (icons, logos, licenses) follow `test-every-change`: stage them from the existing source asset; do not duplicate them in Git.
- Prefer a maintained official or widely used GitHub Action, or the platform's native tool, over a hand-written script that does the same job. Reference it by version tag (for example `@v3`), not by commit SHA. An action that receives secrets is a supply-chain decision: call it out in the summary.
- If existing code already violates this and the replacement cannot be validated with the real platform tools, leave its behavior unchanged and report the violation instead of rewriting it blind.
