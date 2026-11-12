---
id: RR-RP3TNQD
type: review-response
title: Fields checked can go stale before the run lock
finding: 'Cranky 3 and security minor: the handler checks edit rights on the entry''s field set before taking the lock; a sync in between can change the fields the write then acts on.'
severity: significant
resolution: Target gained Fields; CheckResolve and CheckLink refuse with ErrRefused when the entry's refused fields differ from the set that was checked. Pinned in link_test.go.
status: addressed
---
