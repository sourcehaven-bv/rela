---
id: DOCS-VNBHJQ
type: docs-checklist
title: 'Docs: Permission-based navigation filtering'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Exported types and functions have godoc
- [x] Non-obvious decisions explain *why*, not just *what*

`permitsNavEntry` carries the reasoning that matters: why each ACL arm behaves
as it does, and a dedicated paragraph on read-only explaining why the obvious
answer (copy `authorizeCommand`'s deny) is wrong in two directions — the arm was
wrong in the first commit, so the comment now argues the case rather than
asserting it.

## Project Documentation

**Remember `docs/*.md` are GENERATED** from `docs-project/entities/`. Edit the
source guide, run `just docs`, diff. (Learned the hard way in TKT-M1AX6P, where
`just docs` silently reverted ~172 lines of hand-edited output.)

- [x] `GUIDE-data-entry.md` → `docs/data-entry.md`
  - `permission:` in the navigation field table
  - New "Hiding entries a user cannot act on" section: the YAML, the
empty-group rule, the no-policy/read-only behaviour, and an explicit "this is a
convenience, not a security control" paragraph
  - The unvalidated-permission-name gotcha (RR-2KZEXF) with the debugging hint
  - **Fixed a stale table** documenting a `graph:` nav field that does not
exist, while the real `search:`/`settings:` were undocumented (RR-ABO495); also
the example and the counts paragraph
- [x] `GUIDE-acl-security.md` → `docs/acl-security.md`
  - Rewrote § "Sidebar menu structure is principal-independent" so it no longer
contradicts the code: the exception is named, and the three reasons the original
decision gave are restated as *still true* (metamodel not secret, config not
secret, target enforces independently) rather than overturned
- [x] `internal/dataentry/CLAUDE.md` — new section: the filter is presentation
only; do not filter `/_config`; the ReadOnlyACL arm is load-bearing; the switch
stays closed

## Verification

- [x] `just docs && git diff --stat docs/` — regenerated output matches the
committed files; nothing hand-edited survives only in generated form
- [x] `just lint-md` — 0 issues

## External Documentation

- [x] ~~Changelog / release notes~~ (N/A: none maintained in-tree)
