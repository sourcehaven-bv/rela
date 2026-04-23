---
id: RR-5VWLH
type: review-response
title: existingQueryValues silently converts presence-only keys to foo=
finding: internal/lua/urls.go:82-97 parses foo&bar=1 as out['foo']="", which round-trips as foo= (with trailing equals). Cosmetic. Either accept and document the behaviour or preserve presence-only semantics. Low priority; defer.
severity: minor
reason: Presence-only query keys (e.g. foo in foo&bar=1) round-tripping as foo= is cosmetic and touches only author-supplied input to rela.url. No current caller relies on presence-only semantics. Leaving as-is; revisit if a real use case emerges.
status: deferred
---
