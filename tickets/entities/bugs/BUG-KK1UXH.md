---
id: BUG-KK1UXH
type: bug
title: History restore of a deleted entity fails when its status is past the entry state
description: History restore of a deleted entity refused a status past the state machine's entry value; restore now skips the entry rule but still requires a path of held guards to the value and keeps every other write guarantee.
priority: medium
why1: RecreateEntity called Transitions.EnforceCreate, which refuses any state-machine value other than the entry value.
why2: RecreateEntity was modelled as a create, and the entry rule is part of every create path (RR-NB135 added it so a served create could not bypass it).
why3: The rule was written for sync, where a create carried a peer's new record. When sync was removed (TKT-UA1W1L) history restore became the only caller, and nobody re-asked whether a restore is a new record.
why4: The write pipeline has one notion of create. It cannot tell a new record from a record coming back from history, so any rule for new records applies to both.
why5: Rules were attached to the verb (create) rather than to the intent (enter a lifecycle vs. bring back recorded state), and no test pinned the intended outcome of restoring a record past its first state.
prevention: 'Restore has its own state-machine check by construction: RecreateEntity calls statemachine.Set.EnforceRestore (no entry rule; a path of held guards from the entry value must reach the value; unreachable values refused) instead of EnforceCreate. An archguard test (internal/archguard/recreate_test.go) pins every reference to RecreateEntity and every call site so the exemption cannot reach an ordinary create. Regression tests pin restore past the entry state on a faced and a faceless type, the guard and 422 outcomes, and that an ordinary create at a non-entry state is still refused.'
status: done
---

## Problem

`entitymanager.RecreateEntity` (history restore of a deleted face) called
`Transitions.EnforceCreate`. A deleted `done` ticket could not be restored: the
restore returned `ErrIllegalEntry`, a 422 over HTTP and an error from `rela
restore`.

The rule came from sync, where a create carried a peer's new transitions. Sync
is removed (TKT-UA1W1L), so a restore is now the only caller. A restore brings
back recorded history, not an entry transition.

## Decision

Owner ruling: restore is exempt from the entry rule. A restore brings back a
record that existed and was valid; it is not a new creation.

- `RecreateEntity` calls `statemachine.Set.EnforceRestore` instead of
`EnforceCreate`. The code comment there records why.
- **The entry rule does not apply.** A deleted `done` ticket comes back `done`.
- **Transition guards still bind.** Some path of declared edges must lead from
the entry value to the restored value with every `guard:` on it held by the
principal. Holding the guard of the last edge is not enough. Otherwise
delete-then-restore would reach a state a guard keeps the principal out of. A
denial is a 403 naming the first unheld guard on the way, with rule kind
`transition-guard`, and records a `denied-write`.
- **A value no path of declared edges reaches is refused (422).** That
includes a value the schema no longer declares. Such a row could never leave its
state by any transition.
- **`when:` preconditions are not evaluated.** They judge a move from a prior
state against the current graph, and a restore has neither; the cascade delete
has also removed the relations a precondition would usually count.
- **Legality of the move from the deleted state is not checked.** Restoring an
earlier version is a backward move no edge declares (for example `done` back to
`doing`). This is accepted: the trust boundary is the right to delete the entity
and read its deleted history, and a live entity restored to an earlier version
goes through `UpdateEntity`, which does check legality.
- **Guards are evaluated for a deleted entity**, so a grant that came from its
own relations is gone. That fails toward refusal.
- **Automations do not run on restore.** Unchanged: `RecreateEntity` already
suppressed them, because the original create's side effects exist already or
were removed on their own terms.
- Every other guarantee stays: create ACL on `type@face`, the face rule,
validation, the unique check, audit, and versioning. Over HTTP the restore
handler still gates every restored property through `validateFieldWrite`, so an
`options:` grant that excludes the recorded status still refuses the restore
(403).

## Narrowness

Only history restore reaches `RecreateEntity`: the data-entry history restore
handler and `rela restore`. `internal/archguard/recreate_test.go` pins every
reference (`entitymanager.RecreateEntity`, `entitymanager.Recreator` under any
import name, bare names inside the package or a dot-import, and every
`x.RecreateEntity` call) to an allowlist. HTTP create, MCP, Lua and the CLI
`create` cannot reach the exemption. Every ordinary create keeps `EnforceCreate`
in `createCore`.

## Tests

- `TestEnforceRestore`, `TestEnforceRestore_PathNotLastHop` (statemachine)
- `TestTransition_RecreateEntity_RestoresPastEntry` (faceless)
- `TestTransition_FacedRestorePastEntry` (faced: create refused, restore
succeeds)
- `TestTransition_RecreateEntity_GuardedStateIs403`
- `TestTransition_RecreateEntity_UnenterableStatusIs422`
- `TestTransition_IllegalEntryOnCreateIs422` (unchanged, still passes)
- `TestHistoryRestore_DeletedFacePastEntryState`,
`TestHistoryRestore_PastEntryStateStillFieldGated`,
`TestHistoryRestore_PastEntryStateNeedsCreate`,
`TestHistoryRestore_PastEntryStateNeedsGuard` (HTTP)
- `TestRecreateOnlyFromRestore`, `TestRecreateEntryPoints` (archguard)
