---
id: RR-RAQQ3Q
type: review-response
title: ValidateID barely filters code spans
finding: ValidateID accepts plain words, so the shape check does not keep junk out as claimed.
severity: minor
resolution: Claim removed. Junk is harmless because results join live rows; the per-body cap bounds growth.
status: addressed
---
