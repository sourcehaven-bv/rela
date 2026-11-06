---
id: RR-2PPN8XB
type: review-response
title: Undo after soft delete still runs the job
finding: A soft delete fires on.deleted. A restore before the job runs does not cancel it, so an external side effect runs for a live entity.
severity: significant
resolution: The same live-row check drops the job. Documented in GUIDE-metamodel.
status: addressed
---
