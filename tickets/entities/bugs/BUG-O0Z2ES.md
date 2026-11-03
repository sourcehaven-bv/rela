---
id: BUG-O0Z2ES
type: bug
title: Bounded relation replace skipped the audit log on a type mismatch
description: 'writeBoundedFallback wrote a bounded-relation create the type allowlist refuses straight to the store in its own Tx after ReplaceRelations refused it. No write-audit record, relation template or managed order was applied, and the replace was not atomic. GitHub #1806 (remaining part after BUG-X1479P).'
priority: medium
effort: s
why1: writeBoundedFallback wrote the type-mismatched creates with view.CreateRelation inside its own store.Tx, and the store does not audit.
why2: 'ReplaceRelations reported a mismatch as an error and the fallback was built in PR #1803 before RelationOptions.TolerateTypeMismatch existed (BUG-X1479P).'
why3: 'BUG-X1479P fixed writeCreateRelation on develop while #1803 was still open, so the bounded fallback merged with the old shape and the follow-up note in the bug body was not tracked as a ticket.'
why4: Nothing flags a direct store write from a dataentry write path; the rule that all writes go through entitymanager is prose.
why5: A follow-up found in review was recorded as a sentence in a done bug rather than as its own ticket, so it had no owner.
prevention: TestPatchRelations_DisallowedTypeOnBoundedRelationAudited asserts an audit record with the principal for every create and remove on the bounded path. Removing the last dataentry store.Tx write means a soft condition can only be written through the manager. Follow-ups found in review are filed as tickets, not left as a sentence in a done bug.
started: "2026-10-22"
completed: "2026-10-22"
status: done
---

## Description

GitHub #1806, remaining part. BUG-X1479P fixed `writeCreateRelation`. The
bounded path (`writeBoundedOps` in `internal/dataentry/relations_modern.go`)
still had `writeBoundedFallback`: when `ReplaceRelations` refused a create on
the type allowlist (DEC-HWZHA soft condition), it wrote that create straight to
the store through `store.Tx`, without a write-audit record, relation template or
managed order, and then ran a second `ReplaceRelations` for the rest.

Fix: pass `TolerateTypeMismatch` on the creates to `ReplaceRelations`, which
already forwards each create's options to `prepareRelationCreate`. The whole
replace then runs in one manager call and is audited. Remove the fallback and
the now-unused `entitymanager.CheckRelationCapacity` export.
