---
id: RR-11CM
type: review-response
title: Variable name `externalRequests` is misleading
finding: The variable captures ALL http(s) requests, then filters externals at assertion time. Rename to capturedRequests or allHttpRequests.
severity: nit
resolution: 'Variable removed entirely — the fixture uses offOriginRequests (named for what it actually captures: off-origin URLs, not the unfiltered superset).'
status: addressed
---
