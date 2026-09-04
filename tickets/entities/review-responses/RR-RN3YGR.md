---
id: RR-RN3YGR
type: review-response
title: selfHref keeps the bare id for the bare face while _faces[].ref spells it as ID@draft
finding: The reviewer argued the two spellings make a bookmarked /entity/policy/POL-1@draft page write to the published face because servedRef becomes the bare POL-1.
severity: significant
reason: 'Not a defect: writes never take a world, so a PATCH to the bare id always lands on the bare face — the row that page shows. The explicit address stays stable in a bookmark, and goToFace names the default world only for the one navigation that lands on a bare id under a configured default. Changing `_self` for every bare-face response would alter a wire contract (and every attachment href) for no correctness gain; documented in selfHref.'
status: wont-fix
---
