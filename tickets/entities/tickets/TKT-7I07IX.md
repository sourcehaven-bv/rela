---
id: TKT-7I07IX
type: ticket
title: 'visibility: new internal/visibility package — Reader (PolicyReader/AllowAllReader) + tracer decorator + conformance suite'
kind: enhancement
priority: high
effort: m
status: done
---

## Summary

PR 1 of the FEAT-PPH1EU arc (DEC-ZBI39P). Build the shared read-visibility seam;
wire nothing yet.

## Scope

- New `internal/visibility` package, `mayDependOn: acl, affordances, store, entity` (arch-lint: new component block; no cycle — verified in RES-PSZZKU).
- `Reader` interface (narrow, consumer-side): `Get(ctx, type, id)` — row-gate FIRST (hidden and nonexistent indistinguishable, RR-NGMI invariant), then field-redact a **copy** of the entity; `Filter(ctx, candidates)` — batched `PermitsReadMany` by type, fail-closed on gate error.
- Redaction contract hoisted from dataentry's `hiddenProperties`/`copyVisibleProperties`/`stripHiddenProperties`: strip `FieldVerdicts.Visible[name]==false` properties AND apply the display-title fallback (title → ID when the display property is hidden — the secondary-channel leak).
- `PolicyReader`: composes `acl.Request` (reuse ctx-attached via `acl.FromContext`, else `Declarative.ForPrincipal(principal.From(ctx))`) + `affordances.PolicyResolver.FieldVerdicts`. Rejects nil collaborators in the constructor.
- `AllowAllReader`: pass-through, no gate, no redaction. The explicit capability for system jobs (capability ≠ identity; ElevatedManager precedent TKT-D8T148).
- Tracer decorator implementing `tracer.Tracer` over a base tracer: hidden = nonexistent — prune the subtree below a hidden node in `TraceFrom`/`TraceTo` (a hidden node's title never surfaces); `FindPath` through a hidden intermediate → no path, indistinguishable from a real miss; `FindOrphans` drops hidden ids; `HasCycle` must not become an existence oracle (gate the start node).
- Conformance suite modeled on `storetest.RunVisibleSearchTests`: row-gate, field-redaction, title-fallback, subtree-prune, path-withhold-indistinguishability, fail-closed-on-gate-error, NopACL byte-parity, redaction-returns-copy (base entity unmutated).

## Non-goals

Rewiring dataentry/lua (PR 2/3 of the arc). MCP. Any change to `internal/tracer`
itself — it stays pure.
