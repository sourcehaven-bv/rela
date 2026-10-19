---
id: RR-JDOE33
type: review-response
title: The §3 "Ordering inside a round" rationale relies on a check the store does not make
finding: '`channels.Store.Append` does not check state, and `systemPost` reads `Head` before it appends. A manual close, T4''s sweep or a T5b auto-close on another node can land in between, and the post still lands on a closed channel. Document the race, as T3 Post has the same one, or do the state check inside Append.'
severity: minor
resolution: System posts pass an expected-open state to Append, which checks it inside the store and answers ErrNotOpen.
status: addressed
---
