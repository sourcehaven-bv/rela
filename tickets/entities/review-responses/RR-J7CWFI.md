---
id: RR-J7CWFI
type: review-response
title: Neighbour heads ran the per-type gate twice
finding: visibleWorldNeighbors re-ran filterVisible (a second ReadableFacesMany per type) on heads ResolveIDs had already gated, and its comment claimed it cost no store read.
severity: significant
resolution: visibleWorldNeighbors now returns the resolved head ids without a second gate pass; the comment says why. The unused visibleReader parameters were removed from the world neighbour helpers.
status: addressed
---
