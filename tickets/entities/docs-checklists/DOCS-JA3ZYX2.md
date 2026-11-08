---
id: DOCS-JA3ZYX2
type: docs-checklist
title: 'Docs: TKT-0Y566OJ'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Comments where logic isn't obvious: why a small span is listed and not split, the count-sum check on a split, why only the first not_run of an answer counts, the cursor length bound
- [x] Function/type docs if public API: package doc for `internal/reconclient` (protocol, budgets, trust) and `httpremote`; godoc on `Options`, `Result`, `Bearer`, `Header`; comment-lint `doclink` gate clean

## Project Documentation

- [x] ~~README updated (if applicable)~~ (N/A: no project-level change)
- [x] ~~CLAUDE.md updated (if new patterns)~~ (N/A: no new cross-cutting convention)
- [x] Help text accurate (if CLI changes): `rela reconcile diff --help` describes every flag and the credential sources

## User-facing Documentation

- [x] `GUIDE-cli-reference.md`: new `rela reconcile diff` section (view semantics, options, credentials, addresses sent as bounds, plain http refusal, output, skipped types, retries); regenerated with `just docs`
- [x] `GUIDE-acl-security.md`: the problem's `range` member on a refused range; regenerated with `just docs`

## External Documentation

- [x] ~~Changelog entry added~~ (N/A: the release changelog is generated from commits)
- [x] API docs updated (if applicable): `v1.Error.Range` godoc describes the new `range` member
