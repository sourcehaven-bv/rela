---
id: RR-M35RX4
type: review-response
title: Counting mutex race in Tx
finding: Tx copies a fresh mutex over the shared calls map.
severity: minor
resolution: 'Plan: share the mutex through a pointer.'
status: addressed
---
