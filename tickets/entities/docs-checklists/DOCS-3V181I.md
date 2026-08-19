---
id: DOCS-3V181I
type: docs-checklist
title: 'Docs: Gate the membership relation against ACL self-promotion'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Comments where logic isn't obvious (godoc on `Policy.MembershipSelfPromotionOpen`, `RoleDef.IsPrivileged`, `warnUngatedMembership` explain the why: shared-predicate no-drift, privilege gate vs read-only visibility, warning-not-refusal)
- [x] Function/type docs if public API (both new exported methods on `acl.Policy`/`acl.RoleDef` documented)

## Project Documentation

- [x] ~~README updated~~ (N/A: no project-level change)
- [x] ~~CLAUDE.md updated~~ (N/A: no new pattern — follows existing boot-diagnostic + shared-predicate conventions)
- [x] ~~Help text accurate~~ (N/A: no CLI command changes; `rela acl audit` output unchanged)

## External Documentation

- [x] ~~Changelog entry added~~ (N/A: repo has no changelog file)
- [x] API docs updated: `docs/acl-security.md` + `docs-project/entities/guides/GUIDE-acl-security.md` describe the startup warning, when it fires and stays quiet, and the coming world-grant load refusal (TKT-DN37J2)
