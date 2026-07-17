---
id: DOCS-7EL6ZA
type: docs-checklist
title: 'Docs: ACL configurable membership relation (membership_relation:)'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Godoc on `Policy` struct (new `MembershipRelation` field bullet)
- [x] Godoc on `Policy.membershipRelation()` accessor (single-source-of-truth + blank-guard rationale)
- [x] Godoc on `RoleRelationDef` (generalised "Escalation risk for the configured membership relation", member-of as default)
- [x] Godoc on `Policy.Validate` (advisory membership-relation hardening note)
- [x] Seam comment at `readquery.go` HasInbound.Endpoints (RR-1659OA)
- [x] `defaultMembershipRelation` const documented

## Project Documentation

- [x] `docs/security.md` (hand-maintained) — "Hardening the membership relation" generalised + heeft_rol gate example
- [x] `docs-project/.../GUIDE-acl-overview.md` → `docs/acl-overview.md` — `membership_relation:` optional + default + "relation must actually exist" clarification (crit round 1)
- [x] `docs-project/.../GUIDE-acl-security.md` → `docs/acl-security.md` — "Hardening the membership relation" generalised
- [x] `just docs` regenerated; idempotent; `membership_relation` present in generated output
- [x] ~~docs/metamodel.md / cli-reference.md / data-entry.md~~ (N/A: no metamodel/CLI/UI surface change)

## External Documentation

- [x] ~~README / changelog~~ (N/A: small backwards-compatible config addition; surfaced via the ACL guides)

## Verification

- [x] `just docs-check` parity confirmed (generated docs in sync with sources; only pending state is the uncommitted change, which CI resolves on commit)
- [x] Crit review approved (round 2, 0 comments)
