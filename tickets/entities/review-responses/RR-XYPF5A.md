---
id: RR-XYPF5A
type: review-response
title: Striped output locks are held across 30 s writes and consume other channels' claim TTL
finding: 'internal/realtime/local.go outputLocks [64] plus outputs.go:450: a slow output on channel A blocks every channel that hashes to the same stripe, including the HTTP post of an unrelated channel. The inline mark was claimed at c.now, so waiting behind other runs can push it past OutputClaimTTL. The runner on another process then claims it and both run the sets. Fix: use a per-key lock map (refcounted), or claim and refresh ClaimedAt after acquiring the lock.'
severity: minor
resolution: Striped locks replaced by refcounted per-channel locks; holdInline skips claims another runner took and renews past half TTL. TestOutputs_HoldInline, TestLocal_OutputLocksPerChannel.
status: addressed
---
