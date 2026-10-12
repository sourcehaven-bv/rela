---
id: RR-S68UO0
type: review-response
title: Claim Type check has no fixed position
finding: 'Security design review: a Type check after WriteTarget gives ambiguity errors on wrong plurals; after authorize it lets a claim through on the wrong route.'
severity: minor
resolution: 'Plan: Family first; miss or type mismatch returns store.ErrNotFound before WriteTarget and authorize.'
status: addressed
---
