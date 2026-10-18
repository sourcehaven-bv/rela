---
id: RR-S02VXX
type: review-response
title: Test gaps on rate, record and store-error paths
finding: 'Missing: refused take leaves size unchanged; bucket cap by one vs many identities; type record conflict; Create concurrency; storeError with ErrNotFound; unauthenticated caller with realtime off.'
severity: significant
resolution: Tests added for refused take size, one vs many identities at the cap, record conflict, concurrent Create, ErrNotFound mapping and realtime-off with an unauthenticated caller.
status: addressed
---
