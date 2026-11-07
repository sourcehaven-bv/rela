---
id: RR-5GKT001
type: review-response
title: Outstanding entity and relation ETags change at deploy
finding: The encoding changed so clients holding an old ETag get one 412 or a full 200 after deploy.
severity: minor
resolution: Accepted (ETags are opaque) and stated in the PR body.
status: addressed
---
