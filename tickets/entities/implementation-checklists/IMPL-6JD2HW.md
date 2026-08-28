---
id: IMPL-6JD2HW
type: implementation-checklist
title: 'Implementation: Next-action layer Phase 0'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code
- [x] Integration tests written (test full flow, not just units)
- [x] Happy path implemented
- [x] Edge cases from planning handled
- [x] Error handling in place (errors surfaced, not swallowed)

Delivered:

- `internal/dataentryconfig/nextaction*.go` — `next_action_bands:` (ordered,
  operator-declared) and `next_actions:` sources, with validation that rejects
  unknown band references, ambiguous affordance unions, and unknown prominence
  or defer-scope values rather than defaulting them.
- `internal/nextaction/` — the engine: band-order evaluation with
  short-circuit, engine-owned candidate bound, stable-random pick within a
  band, snooze/mute/cooldown suppression.
- `internal/userstate/` — the per-user state seam plus a conformance suite,
  and THREE backends: `memuserstate`, `kvuserstate` (durable, over the
  existing `state.KV`), and `pgstore.UserStateStore` (row-level, for
  multi-process deployments).
- `internal/dataentry/` — the ACL-gated candidate adapter, the
  `/api/v1/_next_action` GET/POST surface, and the option resolver for
  `pick_one`.
- `frontend/` — `NextActionCard`, `NextActionOffers`, the status-bar chip and
  popover, and the shared `useNextAction` composable.

Edge cases that needed deciding rather than coding:

- **Count sources are read-gated by default** (`count_ungated:` opts out). An
  ungated count discloses that entities of a type exist to a principal
  permitted to read none of them.
- **`defer_scope`** — what a snooze covers. Found by walking the research's
  scenarios against a live server: a `pick_one` suggestion keyed per-candidate
  hands back the same suggestion with a different entity.
- **The server owns the key shape.** The handler drops the client's echoed
  `entity_id` for a source-scoped source; trusting it stored the deferral under
  a key the engine never checks — a 204 that silently did nothing.

## Test Quality

- [x] Using fixture builders or factories for test data
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

The user-state contract lives in `userstatetest` and every backend runs it, so
three implementations are held to one behaviour rather than each asserting its
own. Time is injected throughout (`now` is a parameter, never read from the
clock), which is what lets the suite pin TTL boundaries deterministically —
including that a snooze "until T" is over AT T, exactly where `<` and `<=`
diverge silently between a Go map, a JSON file and SQL.

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**

Drove a real `rela-server` against a demo project in the browser for every
prominence tier, and over HTTP for the full scenario set (RES-09YLLL S1/S2/S4/S8):
banner → notice → status-bar `pick_one` (three options from a live query) →
quip → nothing owed.

Three defects were found this way and would not have surfaced from unit tests:

1. **The variant did not round-trip.** A source with `key_props` built a key
   containing a variant the GET never sent, so the client's snooze landed under
   a different key. POST returned 204; the suggestion kept reappearing.
2. **`pick_one` deferral was per-candidate** — see `defer_scope` above.
3. **The handler trusted the client's echoed entity id**, so a source-scoped
   snooze silently did nothing.

Durability was verified by restarting the server: snoozed a suggestion, killed
the process, restarted, and the suppression held. The postgres backend ran its
conformance suite against live PostgreSQL under `-race`, 32 subtests, zero skips.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — the affordance row is one component
      shared by the page surface and the status-bar popover, because a snooze
      that behaved differently depending on where it was clicked is a bug
      nobody thinks to test for. State is a module singleton for the same
      reason: two components resolving independently would double-count the
      impression and could show two suggestions at once.
- [x] No security issues introduced — candidates and `pick_one` options both
      route through the ACL-gated read path; option labels use
      `safeDisplayTitle`, so a redacted display property cannot render a
      partial title (the BUG-R9EHKV leak class).
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind

`just arch-lint`, `just plimsoll`, `golangci-lint run ./...` clean; frontend
typecheck clean with 1706 tests and 0 lint errors.
