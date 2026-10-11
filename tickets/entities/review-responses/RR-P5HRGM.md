---
id: RR-P5HRGM
type: review-response
title: Adopt persists before a later refusal
finding: MigrationState.Adopt records the adopted shape before the candidate is built, so a later refusal leaves the adoption in place.
severity: minor
reason: Adopting the current shape is idempotent and matches what rela migrate baseline would record; documented as a residual.
status: deferred
---
