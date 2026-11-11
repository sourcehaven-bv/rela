---
id: DEC-2NFLAHB
type: decision
title: 'Two-way sync: one side drives and pushes through the normal write API'
context: 'In every two-way case only one side can reach the other: a public client portal holding all clients'' tickets cannot reach an internal rela behind a VPN, members of a family hub reach a central main, a laptop reaches its server. Two one-way pulls (the earlier step 9 plan) need each side to reach the other.'
consequences: 'The driving side needs write access on the other side through that side''s acl.yaml. No new credentials or routes. The write API keeps refusing caller-chosen ids, so the driver''s ledger maps local ids to remote ids. The ledger holds field digests and an ETag per row; a conflict list in the state store holds unresolved conflicts. latest depends on both machines'' clocks and needs a gated last-updated time. Step 9 is split: 9a (push, hold, conflict list) and 9b (latest, local, remote, relation push). Deletes follow later. Step 10 shows the conflict list.'
date: "2026-11-10"
status: accepted
---

## Decision

Two-way sync between two rela instances is driven by one side.

- **One side drives.** The driving side pulls the other side's changes, as a
step 8 pull does, then pushes its own changes through the other side's normal
write API as its existing principal. The credential works as for a pull. rela
issues no credentials of its own (DEC-QKLMMD4): there are no peer keys, no
`peer:` namespace, no routes for remotes only and no new auth mechanism. The
receiving side's `acl.yaml` limits what a push may change. The config and code
say "remote", not "peer".
- **Ids follow the API rules.** The write API refuses a caller-chosen id
unless the type uses manual ids, and sync gets no exception. A new row gets a
fresh id on the receiving side, as in step 8. The driver's ledger keeps the
mapping between the local id and the remote id. Only the driver writes that
mapping, so a remote cannot name a local row it was never linked to. This holds
for one-way and two-way remotes.
- **Share scope.** A push sends only rows of the types the remote lists,
optionally narrowed by a condition per type. The receiving side's own ACL
separates its users, for example the clients on a public portal.
- **Field-level three-way merge.** The driver's ledger keeps the last synced
value of each field as a digest. A field conflicts only when both sides changed
it since the last sync. Otherwise the changed side's value flows to the other
side. A push sends If-Match with the last-seen ETag. A refused push is a
conflict and is never overwritten. On a 412 the driver fetches the row again,
merges again and retries once.
- **Conflict rules.** Each remote in `remotes.yaml` has a conflict rule, with
an override per type or per field: `hold`, `latest`, `local` or `remote`. The
default is `hold`: change neither side and record the conflict on a conflict
list until someone resolves it. `latest` keeps the losing value on the conflict
list and depends on the clocks of the two machines. The last-updated time it
compares is served only when nothing on the row is hidden from the reader;
otherwise that row falls back to `hold`, with the reason on the conflict list.
- **Approval is not sync.** A field mapping (for example a portal `priority`
to an internal `requested_priority`) plus an ordinary workflow covers approval.
- **Echo stop.** After a push the ledger records the pushed value, so the next
pull writes nothing.
- **Deletes** do not travel in two-way sync yet; a follow-up ticket adds them
with guards like `on_gone`.

A hub of several members (a family hub) is several pairs, each member driving
its pair with the hub.
