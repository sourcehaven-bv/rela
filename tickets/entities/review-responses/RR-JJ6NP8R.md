---
id: RR-JJ6NP8R
type: review-response
title: 'Security: a peer that stops reading holds its slot'
finding: The slot was released after writeV1JSON wrote to the socket; with no server write timeout a non-reading peer could pin slots and starve others.
severity: significant
resolution: The ranges run in run(); the slot is released before the answer is written. Pinned by TestReconcile_SlowReaderHoldsNoSlot.
status: addressed
---
