---
id: RR-U31Q7Q
type: review-response
title: 'Three backends emitted three different event streams for one operation'
finding: 'pgstore and sqlitestore emitted a single EventRelationUpdated naming the POST-swap triple, while the generic fallback (going through CreateRelation/DeleteRelationState) emitted a real create and delete pair. An id-keyed consumer told only ''updated'' keeps a ghost edge in the old direction, since the edge at the new triple did not exist before and the one at the old triple no longer does. pgstore''s own tombstone comment states this correctly and then the events contradicted it. The conformance suite was green throughout, because it only compared final row state and counts.'
severity: critical
resolution: 'Both native backends now emit delete-then-create per edge, matching the fallback and the tombstones. A new conformance case asserts the pair on every backend and explicitly fails an Updated op. Asserted as a SET rather than a sequence, because the fallback creates before deleting (so a crash duplicates rather than destroys) while the native path reports the removal first - pinning one order would force a backend into a worse write order to satisfy a test. The sqlite tombstone half of this finding does not apply: the deletions table is postgres-only (it backs the multi-process sync manifest) and sqlite''s ordinary DeleteRelation writes none either.'
status: addressed
---
