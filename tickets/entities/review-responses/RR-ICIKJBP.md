---
id: RR-ICIKJBP
type: review-response
title: 'Nits: exported Headless, ETag on 404, goreleaser duplication, dist/, no local recipe'
finding: Several small points from review.
severity: nit
resolution: Headless is now used by cmd/rela-server so stays exported; ETag hashing of an empty body is negligible (wont-fix); goreleaser entries match existing style without anchors; the snapshot dist/ output was deleted and is not committed; added just test-headless.
status: addressed
---
