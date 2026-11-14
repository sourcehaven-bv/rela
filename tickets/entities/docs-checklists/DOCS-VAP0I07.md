---
id: DOCS-VAP0I07
type: docs-checklist
title: 'Docs: TKT-UO8UIGO'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Comments where logic isn't obvious: fresh Lua state per case, hidden-target denials, principal assignee mapping, fixture symlink refusal, network-attempt flag
- [x] Function/type docs if public API: godoc on `apptest` package, `lua.NewSandbox` module directories, `aclmap.MapPrincipalFull`; comment-lint `doclink` gate clean

## Project Documentation

- [x] ~~README updated (if applicable)~~ (N/A: no project-level change)
- [x] ~~CLAUDE.md updated (if new patterns)~~ (N/A: no new cross-cutting convention)
- [x] Help text accurate (if CLI changes): `rela test` flags (`--run`, `--seed`, `--require-tests`) described in kong help

## User-facing Documentation

- [x] ~~User guide for rela test~~ (N/A: deferred to TKT-E2IMTMG, part 4, which owns the guide and CLI reference; note for it: expect_mail needs `.rela/mail.yaml`, which is gitignored)
- [x] ~~Security guide~~ (N/A: no ACL or read-path change)

## External Documentation

- [x] ~~Changelog entry added~~ (N/A: the release changelog is generated from commits)
- [x] ~~API docs updated~~ (N/A: no HTTP API change)
