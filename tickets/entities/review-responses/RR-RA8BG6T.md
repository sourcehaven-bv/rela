---
id: RR-RA8BG6T
type: review-response
title: Create leaves an orphan entity when its relation fails
finding: createProposed created the entity, then the relation; a refused relation left the entity behind.
severity: minor
resolution: The new entity is deleted when its relation create fails; the error names a failed cleanup.
status: addressed
---
