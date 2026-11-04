---
id: RR-3DFHGP2
type: review-response
title: Mistyped CONTRACT_SEED runs on NaN
finding: Number() of a bad value gives NaN and fast-check runs silently.
severity: nit
resolution: replaySeed throws when the value is not an integer.
status: addressed
---
