---
id: RR-3TRJM5
type: review-response
title: Uncarried-edge check cannot fire for an implicit source
finding: '[security] After the change DeleteFace removes nothing for an implicit source, so the post-delete check never fired there.'
severity: minor
resolution: The move lists the source tail again before DeleteFace and fails if an edge appeared, for implicit and named sources. The post-delete check stays as a backstop and names the edge data.
status: addressed
---
