---
id: TKT-UH2Q9MN
type: ticket
title: 'Two-way sync with a remote: push and held conflicts (9a)'
kind: enhancement
priority: medium
effort: m
started: "2026-11-10"
completed: "2026-11-11"
status: done
---

## Description

Two-way sync with a remote, driven by one side (DEC-2NFLAHB), step 9a. The
driving side pulls the remote's changes (step 8), then pushes its own changes
through the remote's normal write API as its existing principal. The remote's
`acl.yaml` limits what a push may change. rela issues no credentials
(DEC-QKLMMD4).

- `push: true` in `remotes.yaml` makes a remote two-way.
- Ids follow the API rules: a row created by a push gets a fresh id on the
remote. The driver's ledger maps the local id to the remote id.
- Share scope: the types the remote lists, with an optional condition per
type.
- The ledger keeps the last synced value of each field as a digest. A field
conflicts only when both sides changed it since the last sync; otherwise the
changed side's value flows to the other side, per field.
- A push sends POST for a new row and PATCH with If-Match for a changed one.
On a 412 the driver fetches the row, merges again and retries once. A refused
push is a conflict and is never overwritten.
- Conflict rule `hold` only; `conflict:` accepts only `hold` until 9b.
- Held conflicts are stored durably in the state store, reported in the sync
output, and listed and resolved from the CLI.
- Document that approval is a field mapping plus an ordinary workflow.

Out of scope: `latest`, `local`, `remote` and relation push (9b); deletes
(follow-up); status page and conflict UI (TKT-0FMYPEZ).

## Acceptance

- End-to-end test with two instances that each edit different fields and the
same fields of the same entities.
- Echo stop: after a round trip with no new edits, the entity version and the
audit log are unchanged on both sides.
- A conflict is held on both sides and listed; resolving it makes the next
sync apply the chosen side. A refused push is a conflict.

## CI fixes

The push phase runs on its own `pusher` type, so `runner` stays under the
plimsoll method line. A held remote value is kept as `json.RawMessage`, not
`any`, which clears the Archplan untyped-data finding.
