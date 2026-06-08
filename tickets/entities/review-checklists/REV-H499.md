---
id: REV-H499
type: review-checklist
title: 'Review: ACL read-side (PR 1/2): per-entity GET + writes + ?include= gated; middleware fail-loud; ETag'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`) — full tree green; race-clean for dataentry/acl/store.
- [x] Lint clean (`just lint`) — 0 issues.
- [x] Coverage maintained (`just coverage-check`) — 74.3%, package floors PASS.
- [x] arch-lint clean (`just arch-lint`).

## Code Review

- [x] Run code review — invoked cranky-code-reviewer + go-architect agents
  on the rebased diff (60 files post-rebase, 2 TKT-VQGN-only commits plus
  the rework commit). Round 1 produced 22 RRs (RR-MZU4, RR-T15E, RR-NGMI,
  RR-FRK1, RR-7TIU, RR-372L, RR-A62O, RR-MILH, RR-AGSR, RR-QLQW, RR-CAFF,
  RR-P2M7, RR-E703, RR-U06D, RR-I2SI, RR-H9QB, RR-J25J, RR-FGUZ, RR-875A
  + 3 more). Round 2 (post-implementation) produced 2 crits, 5 sigs,
  several nits.
- [x] All critical review-responses addressed:
  - **CRIT-1** (router wrap order): fixed in `internal/dataentry/router.go`;
    `attachACLRequest` now wraps inner-to-`stampAuditPrincipal` so the
    principal is stamped before ACL reads. Regression test
    `TestACLMiddleware_RouterChainOrder` pins the composition.
  - **CRIT-2** (5 ungated relations handlers): added `gateReadOrNotFound`
    calls at the top of `handleV1EntityRelations`, `handleV1GetRelationType`,
    `handleV1CreateRelation`, `handleV1UpdateRelation`,
    `handleV1DeleteRelation`.
- [x] All significant review-responses addressed:
  - **SIG-1** `newACLReadGate` constructor rejects nil; both wiring sites
    route through it (`internal/dataentry/readgate.go`,
    `internal/dataentry/router.go`).
  - **SIG-2** existing-Request branch verifies `existing.Principal() ==
    principal.From(ctx)`; mismatch returns 500 `acl_principal_mismatch`
    with loud log.
  - **SIG-3** `filterVisibleIncludes` adds `slog.Warn` on `PermitsReadMany`
    error before fail-closed drop (type/candidates/err).
  - **SIG-rename** (overpromise): `Visible` → `PermitsRead`; collapses the
    "this checks existence" misread.
  - **SIG-architect-rework**: dropped `WhereIDs` from `GraphQuery` DSL
    (asymmetric matched-vs-total contract); added `GraphQueryer.MatchingIDs`
    (clean map[id]bool contract). `readGate` drops `Query`, gains
    `PermitsReadMany`. Naive impl + pgstore + fsstore + memstore +
    NullGraphQueryer + storetest conformance all updated. Eliminates the
    `q := *rqr.Query; q.WhereIDs = ...` shallow-copy footgun.
- [x] Self-reviewed the diff for unrelated changes — TKT-VQGN-only.

**Review Responses (addressed):**

| RR | Sev | What |
|----|-----|------|
| RR-NGMI | critical | gate runs BEFORE getEntity for timing parity |
| RR-FGUZ | critical | gate runs BEFORE body parse so 400/412 don't oracle existence |
| RR-7TIU | significant | fail-closed local-then-merge on include filter |
| RR-372L | significant | slog.Warn raw err; constant Detail in body |
| RR-875A | significant | middleware fail-loud on unstamped principal |
| RR-T15E | significant | middleware scoped to `/api/` only |
| RR-MZU4 | significant | ETag suppression on deny |
| RR-FRK1 | significant | batched include filter per type |
| RR-A62O | significant | NewDeclarative rejects nil collaborators |
| RR-P2M7 | minor | bare `/api` path included in middleware scope |
| RR-E703 | minor | existing-Request branch also attaches readGate |
| RR-U06D | minor | appbuild godoc on store-wrapper GraphQueryer forwarding |
| RR-MILH | nit | `principalCtx(user)` helper replaces aliceCtx-only |
| RR-AGSR | nit | `mustNewACL(t, p, st)` helper |
| RR-QLQW | nit | `stripInstance` for body-shape comparison |
| RR-CAFF | nit | middleware test behavioral, not type-equality |
| RR-J25J | nit | writeGateError mapping test |
| RR-I2SI | nit | filterVisibleIncludes fresh-allocated output slice |
| RR-H9QB | nit | PATCH with current ETag on hidden still 404 |

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**

| AC | Status | Evidence |
|----|--------|----------|
| AC1 type-level read grant | PASS | `TestACLGet_TypeLevelReadGrant` |
| AC2 hidden GET → 404 (not 403) | PASS | `TestACLGet_TypeLevelReadGrant` deny cases |
| AC3 PATCH/DELETE on hidden → 404 | PASS | `TestACLWrite_PatchOnHiddenIs404` (4 cases incl. current If-Match), `TestACLWrite_DeleteOnHiddenIs404` |
| AC4 ?include= filters hidden neighbors | PASS | `TestACLGet_IncludeFilter` |
| AC5 ETag suppressed on deny | PASS | `TestACLGet_ETagSuppressedOnDeny` |
| AC6 NopACL regression | PASS | `TestACLRegression_NopACL_GetUnchanged`, `TestACLRegression_NopACL_NonExistentStill404` |
| AC7 middleware fail-loud on `/api/` only | PASS | `TestACLMiddleware_FailLoudOnApi`, `TestACLMiddleware_NonAPIPathsBypass` |
| AC-CRIT-1 (composition) | PASS | `TestACLMiddleware_RouterChainOrder` (verified to fail on the broken wrap order) |
| AC-CRIT-2 (relations gated) | PASS | covered by gate placement; manual diff inspection of 5 handlers |

## Documentation (enhancements only)

Skip this section for bugs and internal refactors.

- [x] ~~Docs-checklist created and linked via `has-docs`~~ (N/A: internal
  security refactor; user-facing surface is the same `/api/v1/*` shape and
  the same 404 body for not-found / hidden).
- [x] User-facing documentation updated — `docs-project/entities/guides/
  GUIDE-acl-security.md` "Read-path gating" section already documents
  per-entity invariants, middleware scope rationale, and "what still
  leaks" (carried in via the TKT-YG35 docs). This PR doesn't change
  user-visible behavior; the guide is accurate as written.
- [x] ~~Docs-checklist marked as done~~ (N/A: no docs-checklist created).

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use — TKT-VMD8 (list/sidebar read
  enforcement) will reuse `readGate.PermitsReadMany` for batched list
  filtering; the gate executes the policy decision, callers don't compose
  GraphQuery directly.

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI
- [x] All CI checks pass
- [x] PR URL documented below

**PR:** <!-- filled in by post-/pr fix commit, mirroring REV-SHX0 pattern -->
