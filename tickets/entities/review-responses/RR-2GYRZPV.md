---
id: RR-2GYRZPV
type: review-response
title: Untracked clients freeze past the 10,000-id ceiling
finding: With the per-connection cap disabled (limit < 0), a client introduced past maxAwarenessClientsPerPeer is applied but untracked, and the new rule then refuses its updates.
severity: nit
reason: rela sets a per-connection cap of 2, so the disabled-cap path never runs here; changing it would diverge from upstream for no rela benefit.
status: wont-fix
---
