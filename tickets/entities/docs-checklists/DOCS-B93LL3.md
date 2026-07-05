---
id: DOCS-B93LL3
type: docs-checklist
title: 'Docs: rela acl audit linter'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Package godoc on `internal/aclaudit` (tier model, Validate-vs-audit distinction)
- [x] Godoc on `Finding`, `Severity`, `MetamodelReader`, `Audit`, `ParseSeverity`, `HasAtLeast`, `isPrivileged`
- [x] Each check function documents its rule + rationale (RR-references for the gating decisions)
- [x] CLI `ACLAuditCmd` godoc (advisory default, `--fail-on`/`--exit-code`)
- [x] `Policy.EffectiveMembershipRelation` godoc (single source of truth note)

## Project Documentation

- [x] `docs-project/.../GUIDE-acl-security.md` → `docs/acl-security.md` — "Auditing your policy with rela acl audit" section: what it catches, the two tiers, `--fail-on` thresholds, **production `--fail-on=any` recommendation** (crit)
- [x] `docs/security.md` (hand-maintained) — references `rela acl audit --fail-on=high` as the hardening check
- [x] `just docs` regenerated; idempotent
- [x] `scripts/demo-acl-audit.sh` — runnable e2e demo (crit); auto-run by CI Demos job
- [x] ~~docs/cli-reference.md~~ (N/A: not a generated doc in this repo; the command is documented in the security guide)

## External Documentation

- [x] ~~README / changelog~~ (N/A: surfaced via the ACL security guide)

## Verification

- [x] `just docs-check` parity (generated docs in sync with sources)
- [x] Demo script passes end-to-end; help text (`rela acl audit --help`) shows `--fail-on`/`--exit-code`
- [x] Crit human review round 1 — all comments addressed
