---
id: REV-DKI1T0
type: review-checklist
title: 'Review: Standalone documents: document: as a navigation entry with optional entity_type'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] `go test ./...` — all pass
- [x] `just lint` — 0 issues
- [x] `just arch-lint` — no warnings
- [x] `just plimsoll` — no warnings (see note)
- [x] `just coverage-check` — PASS, total 77.2%
- [x] frontend `npm run test:run` — 1437 passed
- [x] frontend `npm run typecheck` — clean
- [x] frontend `npm run lint` — 0 errors

**plimsoll note:** during implementation the two new handlers pushed `App` from
89 to 91 methods, over its pinned line of 90. Fixed by making them plain
functions rather than raising the directive.

## Code Review

Run with the cranky-code-reviewer agent against the full branch diff, asked
specifically to challenge the ungated-by-default decision and to verify the
claim it rests on.

**The central claim was independently verified as TRUE.** The reviewer traced
`handleV1StandaloneDocument` → `RenderStandalone` → `luaWriteDeps` →
`ReadDeps.VisibleReader = gatedScriptReader(...)` and confirmed there is no
ungated read reachable from a standalone document script (Searcher is raw but
`rela.search` hydrates every hit through `VisibleReader`). So the ungated
default is sound and stays.

Eight findings, all recorded as review-response entities:

| ID | Severity | Status |
|----|----------|--------|
| RR-E8Z1MR | critical | **wont-fix (reversed)** — premise false: config is not a secret |
| RR-THBQQK | critical | addressed — anchored permission gate untested; gate ordering unpinned |
| RR-DYNFSM | significant | addressed — `RenderStandalone` returned a struct it can't populate |
| RR-ZXGPCU | significant | addressed — documents fail open where commands fail closed (documented) |
| RR-R9O8BB | significant | wont-fix (moot) — `hidesNavEntry` deleted with the sidebar filtering |
| RR-P4E9GL | significant | **deferred** → TKT-OGR566 (no concurrency cap on Lua renders) |
| RR-R6SJB8 | minor | addressed — extracted `handleV1AnchoredDocument` (134 → 22 line dispatcher) |
| RR-WINN6Z | nit | addressed — duplicated docs paragraph |

No open critical or significant responses remain.

### Post-review correction (user)

RR-E8Z1MR was **reversed**. The reviewer found `/_config` contradicting my
documented "document names are not enumerable" claim and I fixed the endpoint.
The user pointed out the premise is wrong: `data-entry.yaml` is an
operator-authored file in the repo — routinely public — so its keys, script
paths and permission values are already disclosed. The claim was the defect,
not the endpoint.

Reverted: the `v1.Document` wire type, `visibleDocuments`/`visibleNavigation`
filtering, sidebar filtering, and the two tests asserting concealment. The
`permission:` deny is now a **403 naming the document and permission** instead
of a disguised 404 — actionable for the operator.

**The sidebar filtering contradicted an existing recorded decision** —
`docs/acl-security.md` § "Sidebar menu structure is principal-independent"
already stated the menu is served identically to every principal and named
per-principal hiding as a tightening deliberately not done. Neither planning
nor this review found it; I found it only while verifying a doc reference.
Now guarded by `TestSidebarAndConfig_PrincipalIndependent` and by a new root
CLAUDE.md rule, "The configuration is not a secret; the data is", which
generalizes past this ticket.

The actual confidentiality boundary — the ACL-gated reader bounding what a
document's Lua can read — was never touched by any of this.

**On the deferral (RR-P4E9GL):** the uncapped Lua render path predates this
ticket — an entity-anchored `script:` document with a wide traversal has the
same exposure, and singleflight only ever collapsed identical `(principal,
entry, config)` triples rather than bounding load. Standalone documents raise
the likelihood, not the ceiling. The fix is a shared bounded pool on the render
path (the `internal/cmdexec` precedent), which is a change to shared
infrastructure rather than to this feature. It degrades availability under load,
does not leak data, and per-document `timeout:` bounds any single render.

## Acceptance Verification

Verified against a live `rela-server` on `prototypes/data-entry/project`, with a
real aggregating Lua report and both an ACL-enabled and ACL-free run.

| AC | Result | Evidence |
|----|--------|----------|
| 1 | PASS | Standalone config loads; server starts |
| 2-4 | PASS | Nav validation table tests; valid entry accepted, entity-anchored + unknown rejected |
| 5 | PASS | `GET /_documents/status_review` → 200 with real aggregated content |
| 6 | PASS | standalone+id → 400; anchored w/o id → 400 (both directions) |
| 7 | PASS | Script `assert(rela.document.entry_id == nil)` passed on a live render |
| 8 | PASS | alice (holder) 200 / bob 404, renderer not invoked, deny ≡ unknown-doc |
| 9 | PASS | Ungated document renders for a principal with no permissions |
| 10 | PASS | Sidebar href `/document/status_review`; present for alice, absent for bob |
| — | PASS | No regression: anchored `category_overview/backend` → 200, `entity_ids=['backend']` |

Post-fix re-verification (after the wire-type and extraction refactors):
standalone 200, anchored 200, both kind mismatches 400, unknown 404, sidebar
entry present. `_config` as bob no longer contains `status_review`; neither
principal's payload contains a script path, permission name, or command string.

Browser-verified: sidebar entry beneath Dashboard with a document icon;
click-through and direct deep-link both render; active-route highlight correct;
no Edit button on a standalone document; entity-anchored view and its documents
panel unchanged (the panel offers `ticket_summary` and correctly does not offer
the standalone document).

**Two bugs were found by manual verification that no unit test caught:**
`DocumentView.vue` rendered a dangling "Status Review:" from a hardcoded `{{
docTitle }}: {{ entityId }}`, and the empty state would have read `the entity ""
may not exist`. Both fixed.

## Follow-ups filed

- **TKT-OGR566** — bound concurrent Lua renders with a shared pool (from RR-P4E9GL)
- **TKT-VKM63H** — `navigation:` entries aren't validated for exactly one kind
(pre-existing hole this feature widens by one)
