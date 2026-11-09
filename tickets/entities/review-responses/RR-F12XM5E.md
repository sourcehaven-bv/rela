---
id: RR-F12XM5E
type: review-response
title: Full peer table locks out new principals for the idle TTL
finding: With 1024 principals seen in 10 minutes every new principal got 503, including legitimate peers.
severity: minor
resolution: evictIdle now drops the least recently seen idle principal when TTL eviction frees nothing; refusal needs every tracked principal to hold a slot. Pinned by TestReconcileGate/a full peer table evicts the oldest idle principal.
status: addressed
---
