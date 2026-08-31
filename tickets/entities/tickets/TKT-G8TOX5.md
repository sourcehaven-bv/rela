---
id: TKT-G8TOX5
type: ticket
title: Document why rela import bypasses transition guards
kind: docs
priority: low
effort: xs
status: backlog
---

## Description

`importEntity` writes directly to the store
(`internal/importer/importer.go:423,427` — `imp.store.UpdateEntity` /
`CreateEntity`), bypassing `Transitions.EnforceCreate/Update`. So `rela import`
can place an entity in any status regardless of the metamodel's entry-state
rules.

GitHub issue #1155 proposes enforcing the guards, noting the sync path already
answered the same question with "enforce" (RR-NB135).

## Decision: leave ungated, document why

Decided by the project owner. Two facts settle it, both verified:

**1. The importer is CLI-only.** `internal/cli/import.go:28` is its sole
non-test caller. There is no HTTP, MCP or automation path into it.

**2. A guard here is a speed bump, not a boundary.** An operator running `rela
import` already has filesystem access to the store — fsstore is markdown on
disk, and a pgstore operator has the connection string. Anything the guard would
prevent, the same person can do with a text editor.

**3. Import's job is loading data whose states ALREADY exist.** Enforcing
entry-state rules would reject a legitimate `status: done` record from the
system being migrated from — which is exactly the data an import carries. The
guard would not prevent bad states; it would prevent *true* ones.

## Why this diverges from sync, deliberately

RR-NB135 decided the sync path enforces. That is not inconsistent with this, and
the difference is worth writing down so the next reviewer does not read it as an
oversight:

- **Sync carries ONGOING writes from a peer.** It is a live channel, running
repeatedly, where the peer is not necessarily the local operator. Entry-state
rules are meaningful there because the writes represent new transitions.
- **Import is a ONE-SHOT load run by the operator.** Its records are historical
facts, not transitions being made now. Applying "you must enter at `todo`" to a
record that was `done` five years ago in another system is a category error.

## Precision on the CLI's write paths

Worth recording, because a casual reading of "the CLI writes directly" is wrong
and would make this decision look sloppier than it is:

- `rela create` (`internal/cli/create.go:54`) and `rela restore`
(`internal/cli/restore.go:61,68`) go through **EntityManager** and ARE guarded.
- `rela normalize` (`internal/cli/normalize.go:55`) writes to the store directly
but only rewrites markdown headers — it cannot change `status`, so transitions
never apply.
- `rela import` is therefore the ONLY CLI path that both bypasses the guards and
can set an arbitrary status.

So this is a single, deliberate exception rather than a general CLI posture.

## Scope

IN: document the decision on `importEntity` — why it writes to the store
directly, why that is not a boundary, and why it diverges from sync.

OUT: any change to the importer's write path.

OUT: revisiting RR-NB135. Sync's decision stands on its own reasoning.

## Acceptance

A reader arriving at `importEntity`, or the next IB review, finds the decision
and its reasoning rather than re-deriving the finding. It must answer: why is
this ungated, why does sync differ, and what would have to change for the answer
to change (namely: a non-CLI caller appearing).
