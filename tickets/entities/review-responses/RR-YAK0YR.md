---
id: RR-YAK0YR
type: review-response
title: 'PR 8: count_ungated misses faced-only types'
finding: The ungated next-action count read InWorld, so a type stored only at named faces looked empty.
severity: minor
resolution: countIsZero's ungated branch reads AllFaces.
status: addressed
---
