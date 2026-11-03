---
id: RR-VTL83FN
type: review-response
title: Backend table needs two clarifications
finding: In the postgres build a non-pgstore store gets in-memory channels; rela channel watch already uses Local.Subscribe.
severity: nit
resolution: Build limit stated under the table; rela channel watch now starts a follower (its --poll stays as a safety net).
status: addressed
---
