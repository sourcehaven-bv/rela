---
id: RR-W0WPLZ
type: review-response
title: Every delete commit triggers a full orphan scan and up to eight tombstone listings
finding: RunRetention (internal/realtime/retention.go:179-191) runs on every wake. Each run re-lists all tombstones per pass, calls store.Keys plus existence reads over every channel id, and reads ListMarked once per 500-id chunk (realtime_worker.go:163). A user able to delete entities keeps the leader scanning back-to-back, which costs the most on fs tree walks.
severity: minor
resolution: A delete wake runs RunPurge, which makes one DueTombstones listing and no anchor calls (pinned by TestSweep_Budget).
status: addressed
---
