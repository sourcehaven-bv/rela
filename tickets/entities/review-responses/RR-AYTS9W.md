---
id: RR-AYTS9W
type: review-response
title: pgstore liveEntityHash hand-writes the column list
finding: pgstore/purge.go repeats getEntitySQL by hand.
severity: significant
resolution: Uses getEntitySQL.
status: addressed
---
