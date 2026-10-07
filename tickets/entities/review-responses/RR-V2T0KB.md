---
id: RR-V2T0KB
type: review-response
title: go.mod replaces textanchor with a local path
finding: The replace directive builds against a sibling checkout with no version or go.sum hash.
severity: minor
resolution: textanchor tagged v0.3.0 and pushed; rela requires v0.3.0 and the replace directive is gone.
reason: Temporary during development. Replaced with the v0.3.0 tag before the PR, once the user approves pushing and tagging textanchor.
status: addressed
---
