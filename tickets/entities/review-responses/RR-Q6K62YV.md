---
id: RR-Q6K62YV
type: review-response
title: 'Design: registry release by org id can close a revived tenant under a live lease'
finding: Acquire skips an evicting resident and overwrites residents[org] with a new one; release looks up by org id, so the old lease decrements the new resident. The old store never closes and the new one can close under a live request (registry.go Acquire/release).
severity: critical
resolution: Lease releases through its own resident pointer; an evicting resident still leased is revived by the next Acquire. Pinned by TestRegistry_ReviveEvictingResident and TestRegistry_ClosesEachResidentOnce.
status: addressed
---
