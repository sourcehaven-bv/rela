---
id: RR-NKQA5F
type: review-response
title: Resolver.Address drops the legacy-id fallback; docs hidden{} passes vacuously
finding: Resolver.Address answers a miss for an address entity.ParseRef refuses, where the old Reader.Get read it at the default face via parseAddress. internal/docs seedRowOf still finds such an id through a raw GetEntity, so a hidden{} claim on it passes against any policy.
severity: significant
resolution: The resolver keeps the miss (it fails toward less access, and the dataentry GET already 404s an unparseable address). assert_read.go now refuses an address the grammar rejects with a clear error, so hidden{} cannot pass vacuously. ScriptReader keeps parseAddress until it moves onto the resolver (PR 5).
status: addressed
---
