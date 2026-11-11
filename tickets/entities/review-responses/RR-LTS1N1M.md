---
id: RR-LTS1N1M
type: review-response
title: Run and task lists cut an arbitrary subset
finding: HolderLookup.Headers read up to the cap in store (id) order, and RunList and Tasks sorted only that subset by start time. Past the cap the newest runs or the oldest tasks could be missing.
severity: significant
resolution: HeaderQuery has an Order field that becomes GraphQuery.OrderBy, and the Go fallback sorts the same way before the cut. RunList orders by started descending and Tasks ascending. TestHolderLookup_Headers pins it in both pushdown modes.
status: addressed
---
