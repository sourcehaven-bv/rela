---
id: RR-YTOHZS
type: review-response
title: One damaged fs filing file blocks retention for every batch
finding: filechannels/retention.go:212-215 returns readFiling's error from Tombstones, so one unparsable ~filings/<stamp>.yaml stops all purges. This fails closed but halts erasure project-wide. Skip and log the bad batch instead.
severity: minor
resolution: A damaged filing is logged and skipped, so other batches still progress (TestDamagedFilingIsSkipped).
status: addressed
---
