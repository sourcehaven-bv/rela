---
id: DOCS-OTDO
type: docs-checklist
title: 'Documentation: Create-form field affordances (TKT-3I5U)'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Public APIs documented — `Manager.ValidateCreate` has a contract docstring (no persist/audit/automation, advisory only). `handleV1DryRunCreate` has a header comment covering read-shaped, verdict-only, advisory semantics.
- [x] Non-obvious WHY captured — comments cite RR-R8OR (read-shaped), RR-4O6E (verdict-only), RR-Y85M (advisory), RR-SIA6 (server applies defaults post-gate), RR-2U2D (userTouched), RR-7PL4 (no-store), RR-GOR8 (drift guard), RR-2PZB (unmount guard) at the relevant call sites.

## Project Documentation

- [x] `docs/data-entry/api-reference.md` — added "Create-mode affordances: dry-run (`?dry_run=true`)" section documenting the endpoint, advisory semantics, value-dependent behavior, relations-not-staged scope, fail-open SPA, and the explicit out-of-scope list (list-query, per-link, staged-relations, inline warning display). Removed the now-stale "create-mode affordances stub doesn't gate creates" note.

## External Documentation

- [x] ~~User guide / tutorial~~ (N/A: feature is automatic; policy authors interact with `acl.yaml` not a UI surface).
- [x] ~~Changelog~~ (N/A: project has no separate changelog; commit/PR description carries the user-facing summary).
- [x] CLAUDE.md — no new pattern that warrants a project-rule entry. The staged-entity + dry-run combo is documented in api-reference.md.

## Verification

- [x] Docs are accurate against current code (verified end-to-end browser demo against documented behavior).
- [x] No dead links or stale references in updated section.
