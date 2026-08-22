---
id: RR-S4I9M9
type: review-response
title: Message as opaque pre-rendered bytes leaves no seam for TKT-U2R7GU's per-recipient ACL gate
finding: 'Message carries already-rendered parts, which is architecturally clean but means that by the time bytes reach Sender.Send there is no type-level distinction between content rendered from ACL-gated reads and content rendered from a raw store handle. Mail crosses the ACL perimeter permanently and irrevocably — you cannot un-send. Deferring per-recipient scoping to TKT-U2R7GU is a legitimate scope call, but deferring the SEAM is not: if Message ships as a bag of bytes with no provenance, the follow-up ticket has nowhere to attach the gate and will bolt one on at the call site, which is exactly the per-consumer redaction CLAUDE.md forbids (read-out paths go through internal/visibility decorators injected at the wiring site, never per-consumer calls).'
severity: significant
resolution: Message now carries the principal it was rendered for as a required field, set by the caller. This ticket does not enforce anything with it (there is no trigger yet), but it is the named anchor TKT-U2R7GU attaches its internal/visibility gate to, recorded in the package doc. This ships the known gap with the seam already in place, rather than forcing the follow-up to bolt a gate onto the call site — which is the per-consumer redaction CLAUDE.md forbids.
status: addressed
---
