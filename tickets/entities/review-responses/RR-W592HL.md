---
id: RR-W592HL
type: review-response
title: Restored relation dedups against its own delete version
finding: Relation sweep takes the latest version of any op, so a soft-delete-restored relation (same rel_record_id) is skipped against its cascade delete version and history ends in delete.
severity: minor
reason: 'Not a defect. SoftDeleteEntity writes no version (pinned by TestSoftDelete_RestoreRoundTrip: a mark records no version); PurgeSoftDeleted writes the delete versions only when the rows are gone for good. A history restore of a deleted relation goes through CreateRelation, which mints a fresh rel_record_id. No path leaves a live relation whose lineage ends in a delete version.'
status: wont-fix
---
