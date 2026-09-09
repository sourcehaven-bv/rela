---
id: TKT-FXRPYE
type: ticket
title: Scope the timing claim in the ACL guide to entity-level filtering
kind: docs
priority: low
effort: xs
status: done
---

## Description

`docs/acl-security.md` states that hidden rows produce "no hidden-row work to
measure through timing". That claim is true for ENTITY-level (row) filtering,
which is pushed into the query, but property-level `visible:` redaction runs in
Go after the rows are fetched — so a request touching redacted fields does
measurably more work than one that does not.

GitHub issue #1094. Source: IB-review of rela#1089. Severity: low.

## Decision: document the scope of the claim, do not push redaction into the store

Decided by the project owner. The finding is a DOCUMENTATION imprecision, not a
design flaw, and the correction should say so plainly rather than reading like a
concession.

**Why not push property redaction into the store.** `visible:` grants are
per-principal and can be conditional on graph predicates (`has_relation`,
`count_relations`), evaluated against the requesting identity. Expressing that
as a query pushed to every backend would mean reimplementing the affordance
resolver in SQL for pgstore and again for fsstore and memstore — for a timing
signal measured in the microseconds a redaction loop takes over an already-
fetched row. The cost is a permanent, backend-multiplied correctness burden on
the most security-sensitive code in the tree.

**Why the current design is a strength, not a shortcoming.** rela has a single
central point where entity access is decided, and it enforces both row-level
gating AND field-level visibility. Most web applications have neither: access
checks are scattered across handlers, and field-level redaction usually does not
exist at all — a "hidden" field is one the template happens not to render.

`internal/dataentry/visiblereader.go` makes that concrete: it exists so gating
is STRUCTURAL rather than by convention, holding the store privately and
exposing only gated reads, precisely because "gate by convention" was the
read-ACL bug class (TKT-N26KLB, #1010). Verified: the one remaining raw
`a.store.GetEntity` on the HTTP path (`api_v1.go:2326`) reads an entity only to
compare its TYPE, returns no properties to the caller, and is followed
immediately by an ACL gate.

So a reader arriving at this finding should come away understanding that the
residual timing signal exists BECAUSE the system does field-level redaction at
all — not despite some omission.

## Scope

IN: correct the claim in `docs/acl-security.md` so it says what it actually
covers — entity-level filtering is pushed into the query; property-level
redaction runs in Go and is not constant-time — and record why that is the right
trade.

OUT: any change to where redaction happens.

OUT: constant-time redaction. Worth naming as rejected rather than unconsidered:
padding the redaction loop would trade a real, permanent complexity cost in the
ACL path against a signal that requires an attacker to already hold a valid
principal, already know which field to probe, and to distinguish microseconds
across a network.

## Acceptance

The corrected passage must be precise about which mechanism it describes, and
must not overclaim in the other direction — "not constant-time" is accurate;
"vulnerable to timing attacks" would be alarmist for a signal of this size.

`docs/` is GENERATED from `docs-project/` entities; edit the source.
