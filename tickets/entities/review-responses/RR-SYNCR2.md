---
id: RR-SYNCR2
type: review-response
title: 'Push ordering: defer relations referencing un-adopted temp ids (two-temp-endpoint case; no push against a temp id)'
finding: 'Push creates entities under a temporary local id, adopts the remote-minted id on ack, then pushes relations with endpoints resolved to remote ids. orderForApply already sequences entities-before-relations, but there is no batch transaction (order.go) and push halts per-record on conflict while continuing the run. Failure modes: (a) entity A create push fails/conflicts but relation A->B is still attempted with A under its temp id -> dangling/wrong-endpoint edge; (b) a relation referencing TWO temp endpoints (TEMP_A -> TEMP_B) requires BOTH adoptions before it is pushable, and the remap must patch both endpoints (design mentioned single-endpoint remap only); (c) delete-then-recreate churn from a failed-then-retried relation push can fork rel_record_id version lineage (TKT-92JL8P).'
severity: significant
status: addressed
resolution: 'Push keeps orderForApply (entities before relations). Temp-id adoption uses the manager RenameEntity, which rewrites EVERY incident relation endpoint atomically (RelationsUpdated) — so a relation that referenced a temp-id endpoint is remapped in one call, covering the two-temp-endpoint case without a manual per-endpoint sweep. A relation whose endpoint is not yet on the primary is rejected by the primary endpoint check and resolves on a re-run (idempotent replay), rather than pushing a dangling edge. Lineage-fork risk is avoided because the atomic rename keeps rel_record_id continuous (TKT-92JL8P #1127).'
---

## Finding (design-review, fancy-browser)

Relation push must not run against a temp id. A relation is only pushable once
**all** its referenced endpoints have been adopted (remote ids known). A failed
entity push must **defer** its dependent relations to a later pass rather than
push them against a temp endpoint.

## Recommended resolution

Treat "entity create + its dependent relations" as a dependency unit: push
entities, adopt ids, then push only those relations whose endpoints are all
adopted; defer the rest. Handle the two-temp-endpoint case (remap both `from` and
`to`). Gate relation re-push on stable endpoint ids to avoid delete/recreate
lineage forks. Test: local `TEMP_A -> TEMP_B` relation pushes only after both
endpoints adopted, with both references remapped.
