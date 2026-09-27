---
id: RR-3UTGG1
type: review-response
title: Inheritance closure seeds only default-face rows in pg
finding: pg seeds EntityInheritThrough from e0.face=''; naive walks from e.ID for any face; a face-only entity differs.
severity: significant
resolution: 'The sqlite builder seeds from the ids of the type regardless of face (naive semantics). Correction from code review: the differential harness does not yet generate headless entities, so it did not test this; the pg divergence is filed as BUG-LCHDSR, which also extends the harness.'
status: addressed
---
