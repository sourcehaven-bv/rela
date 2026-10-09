---
id: RR-W592HL
type: review-response
title: Restored relation dedups against its own delete version
finding: Relation sweep takes the latest version of any op, so a soft-delete-restored relation (same rel_record_id) is skipped against its cascade delete version and history ends in delete.
severity: minor
reason: Pre-existing behavior, unchanged by this ticket. Fixing it adds a create after a delete on one rel_record_id, which the relation lifetime readers do not expect; needs its own ticket.
status: deferred
---
