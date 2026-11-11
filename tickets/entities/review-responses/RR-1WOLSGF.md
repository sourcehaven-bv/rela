---
id: RR-1WOLSGF
type: review-response
title: Doc and comment nits
finding: The relsync comment said null is stored as no value; the docs left out the skew slack before the last sync and said local is sent to the remote on pull-only remotes; OpenAPI lacked the relation PATCH 400.
severity: nit
resolution: Fixed the comment; docs/remote-sync.md and the ACL guide now state the slack and that local is sent only with push; OpenAPI lists the 400.
status: addressed
---
