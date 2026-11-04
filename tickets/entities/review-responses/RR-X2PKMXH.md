---
id: RR-X2PKMXH
type: review-response
title: Docs list missing target as a 200 warning
finding: internal/dataentry/CLAUDE.md and api-reference.md still described target_not_found as a soft warning; it is a 422.
severity: minor
resolution: Removed it from the warnings row and table; the id field row and the endpoint bullet now say 422 target_not_found before any write.
status: addressed
---
