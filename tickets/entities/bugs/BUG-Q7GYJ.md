---
id: BUG-Q7GYJ
type: bug
title: rela renumber bypasses Manager — no audit records emitted
description: |-
    Two renumber write paths wrote directly to `store.Store` via `st.UpdateRelation`, bypassing `entitymanager.Manager`, so neither produced audit records:

    1. **`rela renumber` (CLI)** — `internal/cli/renumber.go` wrote each renumbered relation straight to the store. A user-initiated write with no audit trail at all.
    2. **Engine renumber (`maybeRenumberSide`)** — `internal/entitymanager/manager_order.go` rewrites sibling order values to dense ordinals when an `UpdateRelation` collapses spacing. While the triggering update is audited, the cascaded renumber writes themselves were not traceable.

    This violates the architecture rule in CLAUDE.md: *"New write paths inherit audit automatically. Any code that calls entitymanager.Manager.{Create,Update,Delete,Rename}{Entity,Relation} produces a record without further wiring."* Both sites skipped that path.

    **Fix:**
    - **CLI (Site 1):** route writes through `svc.EntityManager().UpdateRelation` with a merge-style `RelationOptions`. This emits an audit record AND applies ACL, consistent with every other write path. Verified live: `rela renumber` now writes `.rela/audit/<date>.jsonl` (previously the audit dir was never even created).
    - **Engine (Site 2):** routing `maybeRenumberSide` through `Manager.UpdateRelation` would recurse (UpdateRelation → runRenumberAfterUpdate → maybeRenumberSide → …), so instead it emits an audit record directly per renumber write — the same approach `cascadeHost` uses for cascade deletes — on a context marked `WithTriggeredBy("renumber:<prop>")`. This makes the cascaded writes traceable and distinguishable from the user write that spawned them, without the re-entrancy hazard.
priority: medium
effort: s
why1: rela renumber wrote relations directly to store.Store, so no audit record was produced for a user-initiated write.
why2: The command predates the audit log and was never migrated to the Manager write path when audit was added; the engine-internal renumber (maybeRenumberSide) likewise wrote to the store directly.
why3: The maybeRenumberSide helper writes to the store deliberately — routing it through Manager.UpdateRelation would recurse, since UpdateRelation triggers the renumber cleanup pass. That correct avoidance of recursion also (incorrectly) skipped audit emission.
why4: There was no single chokepoint that guarantees "a relation write is audited"; audit emission lives inside Manager.UpdateRelation, so any code that writes to the store directly silently opts out — the rule is documented in CLAUDE.md but not mechanically enforced.
why5: The system has two legitimate write tiers (Manager for human intent, store-direct for engine cascades) but only the Manager tier emits audit; cascade-tier writes must each remember to emit a record (as cascadeHost does). Renumber is a cascade-tier write that forgot — the systemic root is that cascade-tier audit is by-convention, not enforced.
prevention: |-
    Regression test `TestRenumber_EmitsAuditRecords` in
    `internal/entitymanager/orderable_test.go` collapses sibling order
    spacing to trigger maybeRenumberSide and asserts that renumber writes
    emit update-relation audit records carrying a `renumber:` triggered-by
    marker (distinct from the user-initiated update).

    The CLI path is covered by routing through entitymanager.Manager, which
    every other audited write path already uses — so it now inherits audit
    and ACL automatically rather than re-implementing a store write.

    Systemic follow-up (tracked separately): the cascade write tier emits
    audit by convention (cascadeHost and now renumber each call
    recordRelationAudit by hand). A shared cascade-write helper that emits
    the record would make the guarantee structural rather than by-convention.
status: done
---

See GitHub issue #886.
