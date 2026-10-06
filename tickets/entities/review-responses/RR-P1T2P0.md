---
id: RR-P1T2P0
type: review-response
title: Redundant Copy before ConnectConfig
finding: pgx.ConnectConfig copies the config itself.
severity: nit
resolution: Dropped the Copy calls and corrected the field comment.
status: addressed
---
