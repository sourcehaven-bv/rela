---
id: RR-4LC9XP
type: review-response
title: Partial indexes built before the bulk hash clear
finding: 0021 maintained the new partial indexes row by row during the bulk UPDATE.
severity: minor
resolution: The UPDATE runs before the partial indexes are created.
status: addressed
---
