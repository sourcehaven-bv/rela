---
id: REV-1VIJR
type: review-checklist
title: 'Review: PATCH entity endpoint silently drops relations payload'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`)
- [x] Lint clean (`just lint`)
- [x] Coverage maintained (62.7% of statements in `internal/dataentry`, floor 55%)

## Code Review

- [x] Ran `/code-review` (cranky-code-reviewer agent) + `go-architect` review in parallel
- [x] All critical review-responses addressed (except one explicitly deferred by the owner)
- [x] All significant review-responses addressed (except the entitymanager-lift, scheduled follow-up)
- [x] Self-reviewed the diff for unrelated changes (the e2e-infra fixes are scoped and called out in the implementation checklist)

**Review Responses:**
- RR-KNXFF — Partial-failure rollback on multi-op writes — **wont-fix** (owner approved the defer)
- RR-8O113 — Reconciler placement — **addressed** (moved to relations.go)
- RR-QF3PX — Raw Go error string leaking — **addressed** (typed relationError + reconcileDetail)
- RR-O4DUL — ETag doesn't cover relations — **addressed** (relations folded into hash, sorted)
- RR-Y7Q6D — UpdateEntity fires on relations-only PATCH — **addressed** (gated on entity changes)
- RR-XD0AP — listRelations swallows errors — **addressed** (listRelationsCtx propagates)
- RR-JWDHH — outgoingRelations ignores ctx — **addressed** (outgoingRelationsCtx accepts ctx)
- RR-XBMBS — Test gaps — **addressed** (5 new tests + 1 happy-path extension)
- RR-UHIF6 — e2e readiness probe masks 403s — **addressed** (sends Origin)
- RR-C0FYK — waitForTimeout race — **addressed** (waitForResponse)
- RR-Q9G5O — metamodel assumption in e2e search — **addressed** (full id)
- RR-HHE0R — stale doc comment — **addressed** (rewrote)
- RR-NEBMQ — no empty-map early exit — **addressed** (len check)
- RR-DZR3I — POST response shape change — **addressed** (grep confirmed no in-tree consumer relies on the old shape; documenting in PR body)
- RR-IXQ5Q — Lift reconcile to EntityManager — **deferred** (follow-up)

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**
- PATCH with `relations` body reconciles outgoing edges — **PASS** (`TestV1UpdateEntity_SavesRelations` covers add, multi-add, shrink, empty-clear, omit, duplicate-ids)
- Only relation types present in the payload are touched — **PASS** (`TestV1UpdateEntity_Relations_ScopedToTypesInPayload`)
- POST with `relations` body creates edges (sibling fix) — **PASS** (`TestV1CreateEntity_SavesRelations`)
- Unknown relation type / target / source-type surface cleanly — **PASS** (`_UnknownType`, `_UnknownTarget`, `_SourceTypeMismatch`)
- Relations-only PATCH does not rewrite entity — **PASS** (`_OnlyPATCH_ETagChangesButEntityStable`)
- Multi-type payload reconciles each type independently — **PASS** (`_MultiType`)
- E2E test drives default RelationPicker and asserts persistence — **PASS** (`forms.spec.ts`: Edit Form - Default Relation Picker Save)
- No regression on the relation-cards widget — **PASS** (relation-cards.spec.ts's 3 pre-existing failures also fail on clean develop; unrelated to this fix)

## Documentation

Bug fix — no user-facing docs changes. N/A.

## Final Checks

- [x] Commit message explains the why (drift between frontend DynamicForm and backend DTOs)
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] ~~Run `/pr` command to create PR and monitor CI~~ (N/A: owner will run /pr separately after landing; this ticket can be marked done once the diff passes local gates)
- [x] ~~CI checks pass~~ (N/A: same — CI runs on the PR the owner creates)
- [x] ~~PR URL documented below~~ (N/A: PR not yet created; user requested review-first workflow)

**PR:** pending
