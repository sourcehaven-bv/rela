---
id: TKT-GGQ0JT
type: ticket
title: 'ACL read-side: close the /_search match-on-hidden-field oracle (drop hits matching only visible:-hidden fields)'
kind: enhancement
priority: medium
effort: m
status: backlog
---

## Problem

`/_search` already redacts the *body* of a `visible:`-hidden property, but the
search index (`internal/search/filter.go` indexes every property value) still
**matches** on the hidden field's text. A query whose only match is a hidden
field still returns the entity as a hit — so the hit's *presence* confirms the
hidden value even though its value is stripped from the response body. Example:
searching a candidate postcode against a hidden `address` field turns search
into a guess oracle. Entity-level gating (`executeQuery`) does not catch this —
the entity itself is readable; only one of its properties is hidden.

Established during RES-H5AB7S. Related deferred note: RR-WX77.

## Direction (from RES-H5AB7S, Option B1)

Close it at the `search.VisibleSearcher` seam: after entity-level scoping,
determine the fields the principal may see for each surviving hit and drop the
hit if its match set ⊆ hidden fields.

- Generic impl (bleve + linear): re-check candidate field text in-process,
bounded by the existing candidate window.
- pgstore-native: build the visible-field projection into the trgm/tsvector
predicate so the match is computed over visible columns only (column- projection
pushdown) for the **static** (predicate-free) `visible:` case; fall back to
bounded per-candidate evaluation where a `when:` predicate makes visibility
entity-dependent. `log()` the fallback (mirror the bleve-10k-floor disclosure in
TKT-BA8BSX).

## Conformance

Extend `storetest.RunVisibleSearchTests` with field-visibility cases: a hit
matching only a hidden field is dropped; same no-leak + ordered-subsequence
invariants, now over the visible-field projection. Run ×3 backends
(generic+memstore, generic+bleve, pgstore-native DB-gated).

## Performance contract

Redaction fieldset cached per (principal, type) per request (same memoisation
shape as `acl.Request` GlobalRoles), except where a `when:` predicate forces
per-entity evaluation. Pushdown applies only to the static-verdict case.

## Reference files

- `internal/search/types.go`, `internal/search/visible.go` — VisibleSearcher seam
- `internal/store/pgstore/visiblesearch.go` — pgstore-native SearchVisible
- `internal/store/storetest/visiblesearch.go` — conformance suite
- `internal/dataentry/helpers.go` (`executeQuery`), `api_v1.go` (`handleV1Search`)
- `internal/affordances/resolver.go` — the visible-field verdict source
