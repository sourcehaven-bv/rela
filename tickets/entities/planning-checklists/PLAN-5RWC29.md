---
id: PLAN-5RWC29
type: planning-checklist
title: 'Planning: Remove sidebar entity counts (badges, ACL-scoped counting path, docs)'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:**

IN: the per-list/per-kanban count badge in the data-entry sidebar and everything
that existed only to produce it — the `sidebarCounts` counting path in
`views_handler.go`, the `Count` field on the `/api/v1/_sidebar` wire type, the
SPA badge markup/CSS/type field, the count-only tests, and the documentation
describing the feature.

OUT: nav `permission:` filtering (TKT-TXDK8U) and its tests; the sidebar icon
behaviour; the read-gate machinery itself (`ReadQuery` still backs the list
pipeline); the `collapsed` nav-group field. Dashboard `display: 'count'` cards
are a different feature and are untouched.

**Acceptance Criteria:**

1. `GET /api/v1/_sidebar` emits no `count` key for any nav item — pinned by the
wire type no longer carrying the field (compile-time).
2. The SPA sidebar renders no count badge; `typecheck` passes with `count`
removed from the `SidebarItem` type.
3. No entity counting runs on a sidebar request — `sidebarCounts` and its three
methods are gone, so no `GraphCount`/`GraphQuery` call remains on that path.
4. Nav `permission:` filtering and icon resolution still behave exactly as
before — existing `TestNavPermission_*` and `nav_icon_test.go` stay green.
5. No doc still describes a count badge or the sidebar count gating.

## Research

- [x] ~~For larger features: run `/research`~~ (N/A: removal of an existing feature, no approach to survey)
- [x] ~~Searched for existing libraries~~ (N/A: deletion, nothing to build)
- [x] Checked codebase for similar patterns or reusable code
- [x] ~~Looked for reference implementations in other projects~~ (N/A: deletion)
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A — small deletion, approach is mechanical.

**Existing Solutions:**

- Prior art for the removal shape: [[TKT-PZTP1L]] (remove dead UIState
persistence) — same "delete a whole dead layer + correct the docs that described
it" pattern, including keeping the `collapsed` wire field for compatibility.
- The feature being removed was built by [[TKT-VMD8]] (ACL read-side PR 2/2);
its review-responses RR-2O27, RR-BZ4M, RR-REQW, RR-OBXBWL are the record of the
complexity and perf cliffs this deletion retires.
- A full-surface inventory was run before editing (Go, tests, frontend, e2e,
docs) to make sure nothing was left dangling.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**

Delete the feature bottom-up: wire type field → server counting path → call
sites/signature → SPA markup, CSS and type → count-only tests → docs. Drop
`ctx`/`counts` params from `navEntryToSidebarItem` since neither is needed once
counting is gone; `slog` becomes unused in `views_handler.go` and is removed.
Fix the three comments that asserted count behaviour (`app.go`, `readgate.go`,
`nav_permission_test.go`) so no prose outlives the code it described.

Alternative considered and rejected: keep the endpoint field and stop populating
it. Rejected — a permanently-null field is a wire-level footgun that invites a
future consumer to read it, and it leaves the counting code alive.

**Files to modify:**

- `internal/apiwire/v1/responses.go`
- `internal/dataentry/views_handler.go`, `app.go`, `readgate.go`
- `internal/dataentry/acl_sidebar_test.go` (delete), `api_v1_test.go`,
`nav_icon_test.go`, `nav_permission_test.go`
- `frontend/src/components/common/Sidebar.vue`, `frontend/src/types/config.ts`
- `docs/{data-entry,acl-security,server-security}.md` and the matching
`docs-project/entities/guides/GUIDE-*.md` sources

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**

No new input. The change removes a read path; it accepts nothing it did not
accept before.

**Security-Sensitive Operations:**

The deleted code was itself security-sensitive: it aggregated over ACL-gated
rows, so it had to guarantee the cardinality of hidden entities never showed
through (TKT-VMD8 AC6/AC7). Deleting it removes that leak surface entirely
rather than shrinking it — a strictly-safer direction.

The one thing to verify is that the deletion does not weaken a *shared* gate. It
does not: `readGateFromContext`/`ReadQuery` remain, still used by
`scopedSortedEntities` for lists; only the sidebar's own call site is gone. Doc
claims that sidebar counts are gated were removed rather than left to rot, since
a stale security claim is worse than none.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:**

- AC1/AC3: compile-time — removing the `Count` field and `sidebarCounts` makes
any surviving reference a build failure; `go build ./...` + `go vet ./...` are
the check. Count-only tests are deleted, not rewritten.
- AC2: `npm run typecheck` (field removed from the type) and the existing
frontend unit suite.
- AC4: existing `TestNavPermission_*` (12 tests, permission filtering) and
`nav_icon_test.go` must stay green unmodified apart from the call signature.
- AC5: grep for `sidebar count` / `nav-count` / `listCount` / `kanbanCount` /
`sidebarCounts` across `docs/`, `docs-project/`, `internal/`, `frontend/src`
returns nothing.

**Edge Cases:**

- A nav entry whose list/kanban id does not resolve in config: previously took
the `if ok` branch and got no count. Now no branch exists — the item renders
with label/href/icon only. No behavioural difference for the user.
- Groups whose items are all permission-filtered: still dropped rather than
rendered as bare headings (unchanged, pinned by `TestNavPermission_*`).
- Action entries (no href): unaffected, never carried a count.

**Negative Tests:**

Nothing new to reject — this removes a response field rather than adding an
input. The relevant negative property (a deny-all principal must not learn
hidden cardinality) is now unconditionally true because no count is computed or
sent, so it needs no test to hold it.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**

- *An API consumer reads `count`.* Low: `/api/v1/_sidebar` is consumed by
rela's own SPA, which is updated in the same change. The field was `omitempty`
so consumers already had to tolerate its absence.
- *Deleting tests lowers coverage below a floor.* Mitigated by running
`just coverage-check`; the deleted tests covered the deleted code, so the ratio
holds (verified: floors pass, 77.4% total).
- *Stale prose survives the code.* Mitigated by a full-surface inventory across
Go, frontend, e2e and both doc mirrors, plus a final grep.

Effort: s.

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] ~~Docs-checklist will be created when entering implementation~~ (N/A: `kind: chore`, not an enhancement; doc edits are removals of text describing a deleted feature and are covered in this ticket)

**Documentation Impact:**

- [x] `docs/data-entry.md` — removes the two lines describing the count badge
- [x] `docs/acl-security.md` — removes the "Sidebar counts go through the same
gate" paragraph and the sidebar config-filter performance caveat; trims sidebar
counts from the aggregate-gating list
- [x] `docs/server-security.md` — trims sidebar counts from two read-filtering
lists
- [x] The three matching `docs-project/entities/guides/GUIDE-*.md` sources, so
the generated mirrors stay in sync
- [ ] ~~CLAUDE.md~~ (N/A: its sidebar mentions are about `permission:` gating and layout, no count references)

## Design Review

- [x] ~~Run `/design-review` before starting implementation~~ (N/A: `kind: chore` deletion of a feature the user asked to remove; no design space — the alternative, keeping a null wire field, is recorded and rejected under Approach)
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** None — no design review run (see above).
