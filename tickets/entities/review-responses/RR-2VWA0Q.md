---
id: RR-2VWA0Q
type: review-response
title: ctx-carried store.Attribution is a new hidden-dependency route into the store; contract must be pinned by tests
finding: 'store.go documents that the store never learns the Principal by any route other than boundary-populated VersionInput. store.WithAttribution/AttributionFrom introduces a second ctx-based route whose failure mode is silent: a write path that forgets WithAttribution stamps NULL and degrades to version-sweep attribution with no error. Mitigations: keep entitymanager as the only populator; absent attribution must map to NULL (never a default principal); add DB-gated store tests pinning both directions (with attribution → columns stamped; without → NULL); update the CLAUDE.md invariant wording to name the Attribution ctx carrier as the second sanctioned boundary-populated input.'
severity: critical
resolution: 'Plan pins the contract: entitymanager is the sole populator; AttributionFrom returns zero-value (never a default principal) when absent; DB-gated tests assert both directions (with attribution → columns stamped, without → NULL); CLAUDE.md content-versioning bullet updated in the same PR to name the Attribution ctx carrier as the second sanctioned boundary-populated input.'
status: addressed
---
