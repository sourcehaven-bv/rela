---
id: RR-YDA4
type: review-response
title: Third test omits `.entity-id` assertion present in the first two
finding: The `falls back to the entityId on the title line when title is omitted` test asserts `.entity-title` text but not `.entity-id` text. The first two tests in the block assert both lines for symmetry; mirroring the assertion here would pin that the ID line still renders regardless of how `title` is absent.
severity: nit
resolution: Added the missing `.entity-id` assertion to the `falls back to the entityId on the title line when title is omitted` test to match the symmetry of the first two cases.
status: addressed
---
