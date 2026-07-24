---
id: DEC-ZBI39P
type: decision
title: Read-side ACL enforcement via visibility decorators over ungated pure readers (hidden = nonexistent)
context: 'PR #1188''s export leak showed field-redaction is enforced by convention at each consumer; Lua reads a raw store and the tracer is ungated everywhere. RES-PSZZKU surveyed the options. The codebase already has one proven shape: search.VisibleSearcher — an ungated base service wrapped by a visibility-filtering implementation of the same contract, conformance-tested.'
consequences: 'Base services stay pure and ACL-unaware (store, tracer, search — per CLAUDE.md); enforcement is structural via injected wrappers, not per-consumer calls. New internal/visibility package hosts Reader (row-gate + field-redact + copy + title-fallback over store reads) and a Tracer decorator implementing tracer.Tracer. Decorator semantics: a hidden node is NONEXISTENT for the principal — trace subtrees below a hidden node are pruned, FindPath through a hidden intermediate returns no-path indistinguishably (RR-NGMI invariant), FindOrphans drops hidden ids. Lua bindings stay unchanged; wiring sites choose PolicyReader/visible-tracer (request paths) or AllowAll (system jobs). Identity stays separate from capability: jobs keep a genuine system:* principal for audit; allow-all is the injected reader, never the identity (ElevatedManager precedent, TKT-D8T148). The redacting reader is read-out only — write-prep reads keep the raw store or hidden fields would be clobbered on save. Costs accepted: find_path semantics change under ACL; per-node gate cost on trace output filtering; ExecuteDocument needs the request principal threaded before export redaction can work.'
date: "2026-07-24"
status: accepted
---

## Decision

Enforce read-side ACL (entity row-gating + field-level `visible:` redaction)
through **visibility decorators wrapped around ungated pure readers**, never
inside the base services themselves.

The pattern, generalizing `search.VisibleSearcher` (TKT-BA8BSX):

| Base (ungated, pure) | Visibility wrapper |
|---|---|
| `search.Searcher` | `search.VisibleSearcher` (exists) |
| `store.Store` reads | `visibility.Reader` — `PolicyReader` / `AllowAllReader` |
| `tracer.Tracer` | `visibility` tracer decorator (implements `tracer.Tracer`) |

## Key properties

- **Structural, not by-convention**: consumers receive a wrapper via wiring; they cannot forget to filter. Lua's trace bindings keep their `tracer.Tracer` dependency untouched — only the injected implementation changes.
- **Hidden = nonexistent**: subtree pruned below a hidden trace node; `FindPath` through a hidden intermediate → no-path, indistinguishable from a real miss; hidden orphan ids dropped. No topology or existence oracle.
- **Capability ≠ identity**: system jobs run under a genuine `system:*` principal (audited honestly) and receive an `AllowAllReader` as an explicitly-wired capability — the read-side analog of `WriteDeps.ElevatedManager`.
- **Read-out only**: the write-prep read path (entitymanager diffing) keeps raw store access; redacting it would clobber hidden fields on save.
- **Fail-closed + NopACL byte-parity**: gate/resolver errors hide, never reveal; without acl.yaml the nop wrapper is byte-identical to today.

Full survey and rejected alternatives (ACL-aware tracer, per-consumer adapters,
store-level redaction): RES-PSZZKU.
