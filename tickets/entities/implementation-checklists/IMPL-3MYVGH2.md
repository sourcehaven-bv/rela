---
id: IMPL-3MYVGH2
type: implementation-checklist
title: 'Implementation: Two-way sync: send deletes with guards'
started: "2026-11-12"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code
- [x] Integration tests written (test full flow, not just units)
- [x] Happy path implemented
- [x] Edge cases from planning handled
- [x] Error handling in place (errors surfaced, not swallowed)

## Test Quality

- [x] Using fixture builders or factories for test data
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**
- `internal/remotesync/deletes_test.go` (15 tests): send, dry run, off by
  default, turned off before the send, gone on both sides, remote edit under
  hold/local/remote/latest, resolve local and remote, 403 refusal, 412 race,
  guard holds only deletes while the pull applies, 24 h window, emptied
  here, restore before send, remote delete of an edited row under each rule,
  `send_deletes` without `push` is a config error.
- `internal/dataentry/delete_ifmatch_test.go`: DELETE entity and relation
  with stale, matching and absent If-Match; the compare ignores hidden fields.
- `internal/dataentry/remote_delete_e2e_test.go`: two instances over HTTP.
  A local delete removes the relation and the entity on the remote after
  `--allow-gone` (emptied-type guard); a remote edit holds the delete as
  `local_deleted`; a remote ACL without delete holds it as refused (403).
  This test found that `httpremote.Send` refused DELETE; fixed.
- Web gate: taking remote on `remote_deleted` needs the delete right
  (`TestRemotes_RemoteDeletedNeedsDelete`). CLI output tests for the new
  reasons and counts.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
