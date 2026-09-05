---
id: RR-X2GBWJ
type: review-response
title: Parse the address once in the dispatcher and pass entityRef, not string, to every handler
finding: Handlers still receive `entityID string` and each decides whether to parse it; a new handler can forget, and the fsstore key collision masks the mistake locally while pgstore 404s. A guard test over store.GetEntity/reader.getEntity call sites, and a round-trip property test over every emitted address, would make the class of finding structurally unavailable.
severity: minor
reason: Larger refactor of handleV1DynamicRoutes and every entity handler signature than this fix should carry; every surface the SPA reaches now parses the address. Tracked on TKT-5SZG2L as follow-up hardening.
status: deferred
---
