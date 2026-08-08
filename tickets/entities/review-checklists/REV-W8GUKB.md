---
id: REV-W8GUKB
type: review-checklist
title: 'Review: Condition-hidden field is pruned on save and can never be revealed again (silent data loss)'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] `go build ./...` clean; `go test ./internal/...` all pass
- [x] `just lint` — 0 issues
- [x] `just arch-lint` — OK, no warnings
- [x] `just plimsoll` (god-object lint) — clean
- [x] `just coverage-check` — PASS (total 77.2%, all package floors satisfied)
- [x] `npx vue-tsc --noEmit` — clean
- [x] `npm run lint` — 0 errors
- [x] `npm run test:run` — 88 files, **1455 tests pass**
- [x] Full e2e suite — **236 passed**, 0 failed, 8 skipped (pre-existing)

## Code Review

Two independent adversarial reviewers on the actual diff: one on frontend
correctness (the async gate, watcher, retention lifetime), one on backend / ACL
/ wire contract.

**7 findings, all addressed.** 1 critical, 4 significant, 2 minor.

| ID | Severity | Summary |
|---|---|---|
| RR-LFXFGW | critical | Post-await clear/revert acted on server-replaced state |
| RR-F3QT0H | significant | Redacted fields writable in section inline-edit |
| RR-G0WF8A | significant | Blanket suppression swallowed coalesced edits; unperformable revert |
| RR-Z1LUO5 | significant | Stale retention across entity switch; reusable `lastEdit` |
| RR-GSXOZ5 | minor | Three stale ACL doc comments; `[object Object]` in prompt |

The critical one matters most: my own fix reintroduced BUG-FB0LN8's failure mode
through the async gap it added. If the form was re-hydrated from the server
while the confirm dialog was open, clicking "Clear" unset whatever value had
*since* arrived — destroying data the user never saw and never approved. Fixed
with a `loadGeneration` fence plus per-value verification: a decision about
state that no longer exists is abandoned, and only the exact values shown to the
user can be cleared.

RR-F3QT0H was a regression introduced by this commit's own tombstone change, and
the fix location matters: it went in the **shared** `isFieldWritable` helper
rather than per-consumer, because one consumer compensating while another did
not is precisely what caused the gap.

## Verification of acceptance criteria

All verified end-to-end against the real `rela-server` binary and a real
browser, and **proven to fail against the pre-fix component** (5 of 6 new e2e
tests fail when `DynamicForm.vue` is reverted; the one that passes is the test
asserting the old opt-in behavior).

| Criterion | Evidence | Result |
|---|---|---|
| Hidden branch keeps its stored value | `edit mode KEEPS a field whose branch is hidden` | PASS |
| Reveal restores the field AND its value | `a hidden field reappears with its value…` | PASS |
| Survives a full page reload | `a hidden-then-revealed field survives a page reload` | PASS |
| `clear_when_hidden: yes` clears | `clear_when_hidden:yes clears the value…` | PASS |
| `confirm` + decline keeps value and reverts trigger | `clear_when_hidden:confirm keeps the value when declined` | PASS |
| `confirm` + accept clears | `clear_when_hidden:confirm clears the value when accepted` | PASS |

## Regression checks

- [x] RR-U9ERK error-clearing preserved (`hiding an errored step clears its flag`)
- [x] RR-O4SRG create-path prune intact (2 tests)
- [x] Relation prune intact
- [x] RR-FD1B read-only row invariant preserved (`computeVisibleFieldAffordances`)
- [x] ACL value secrecy: no hidden value leaks on any path. Reviewer traced
every serializer path (`forWire`, `forWireRelated`, includes, search, dry-run,
transitions, attachments) and confirmed tombstone and `stripHiddenProperties`
derive from the same `Visible` map, so they cannot disagree. Tests tightened to
assert the VALUE never appears.

## Out of scope / deferred

- RR-O0KRI2 (wont-fix, justified): old binaries silently ignore
`clear_when_hidden`. Unfixable *from* this change. Follow-up filed as
**TKT-QHF4JQ** for recursive nested-key config validation.
