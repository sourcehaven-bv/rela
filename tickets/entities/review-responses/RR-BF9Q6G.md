---
id: RR-BF9Q6G
type: review-response
title: SPA comment documenting the old rule was left uncommitted
finding: '`worldForFace` in frontend/src/stores/schema.ts carried a comment block describing the DISTINGUISHABLE-worlds case (same head, different `otherwise:`) as one the server accepts without a declaration. After this change the server refuses it, so the comment documented the old rule as current. The correction was made but left uncommitted in the working tree, meaning commit f08ffe47 — the tree CI would have tested — did not contain it.'
severity: significant
resolution: 'Corrected both comment blocks in `worldForFace`: the doc block now states the function is total against a current server for any face some world heads, and records that the previously-reachable ambiguous branch was the defect. The inline block no longer describes the exempt case as accepted. Committed together with the rationale rewrite so the tree CI tests is the tree that was reviewed. Frontend test:run (47 pass), typecheck and eslint all clean.'
status: addressed
---

The consumer this whole ticket exists to unblock is `worldForFace`, so its own
comment claiming the opposite of the new rule was the worst place for the
staleness to sit.
