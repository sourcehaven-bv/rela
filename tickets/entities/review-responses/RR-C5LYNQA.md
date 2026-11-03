---
id: RR-C5LYNQA
type: review-response
title: Final flush can abort its own saves
finding: commitImmediately's 10 s timeout can trip while the drain advances the clock; pass a large timeout.
severity: minor
resolution: commitImmediately is called with a timeout of 1e9 ms.
status: addressed
---
