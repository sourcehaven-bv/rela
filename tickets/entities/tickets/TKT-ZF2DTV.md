---
id: TKT-ZF2DTV
type: ticket
title: 'lua: ReadDeps reads through visibility.Reader + visible tracer; scheduler jobs get explicit AllowAllReader; prove one role-scoped job'
kind: enhancement
priority: high
effort: l
status: done
---

## Summary

PR 3 of the FEAT-PPH1EU arc (DEC-ZBI39P). Every Lua **read-out** becomes
ACL-bound by construction: swap the raw deps for visibility wrappers at the
wiring sites. This is the enabler for **scheduled ACL-bound LLM jobs** — a job
role's reader bounds what can enter a prompt.

## Survey findings (2026-07-24, branch feat/transform-registry)

A full map of the seam surface was taken before planning. Four findings change
the shape of the work:

1. **The minimal seam is 3 methods, not `store.Store`**: `GetEntity`,
`ListEntities`, `ListRelations`. Lua calls nothing else — no `CountRelations`,
no `Search` via Store, no writer methods. (`store.EntityQuery` /
`store.RelationQuery` stay in the signatures, so `internal/store` remains a type
dependency either way.)
2. **`luaUpdateEntity` (`runtime.go:1459`) reads through `deps.Store` on the
WRITE path** — `GetEntity` → `Clone()` → merge caller props → save. Routing THIS
read through a redacting reader would drop hidden properties from the clone and
**erase them on save**: silent data destruction, precisely the "read-out only"
hazard DEC-ZBI39P names. **It must keep a raw handle.** This is the single most
important constraint in the ticket.
3. **`rela.search` gates only at hydration.** `luaSearch` (`runtime.go:1382`)
gets hits from the un-gated `Searcher`, then re-fetches each hit via
`Store.GetEntity` (`:1388`). A Store-level seam therefore redacts the *bodies*
but the *hit list* still reveals which entities matched — the
match-on-hidden-field oracle RES-H5AB7S/TKT-GGQ0JT describe, now on the Lua
surface. Either route Lua search through `search.VisibleSearcher` or document
the residual explicitly; do not let a redacted body imply a gated hit list.
4. **Validation is unaffected by the seam for its primary read.** The entity
under evaluation arrives as a *parameter* (`validation/lua.go:160` sets the
`entity` global from the caller's already-loaded entity, via `validator`'s own
`EntityReader` — not via `deps.Store`). `deps.Store` in that path serves only
incidental cross-entity lookups inside a rule body. So "validator keeps
AllowAll" is about rule-body lookups, not about the entity being validated.
Related precedent: `validator.go:170-176` skips locked/unreadable entities
because absent values manufacture false "required field missing" violations —
the same argument applies to redacted reads in rule bodies.

Two additional construction sites the original scope missed:
`internal/docs/runtime.go:155` (docs runtime, memstore-backed, nil Searcher) and
`internal/script/luascriptrunner.go:78` (per-cascade WriteDeps re-wrap, also the
`ElevatedManager` bypass site). Full site list: appbuild.go:189/:203/:680,
dataentry/app.go:302/:441, docs/runtime.go:155, script/luascriptrunner.go:78.

## Scope

- **Seam shape**: give `lua` a narrow, locally-defined read interface (the
package already does this twice — `lua.Mutator` in `deps.go:40`, `cacheStore` in
`runtime.go:112`, both with explicit call-site-interface justifications). Decide
between (a) a local 3-method interface satisfied by both `store.Store` and a
visibility adapter, or (b) importing `visibility` and adding it to
`lua.mayDependOn`. Option (a) keeps the arch-lint rule untouched and matches the
package's existing idiom — prefer it unless the adapter proves awkward at the
wiring sites.
- **Split read-out from write-prep in `ReadDeps`**: the redacting handle serves
`get_entity` / `list_entities` / `get_relations` / `search` hydration and
`markdown.go:2367`; `luaUpdateEntity` keeps a raw handle (finding 2). Name the
two fields so the distinction is impossible to miss, and godoc the hazard.
- `Tracer` keeps its `tracer.Tracer` type — `visibility.VisibleTracer` is
drop-in (it implements all three methods Lua uses: TraceFrom/TraceTo/FindPath).
Wiring injects the decorator; **no binding changes at all**.
- `get_relations` uses `Reader.FilterRelations` (both-endpoints FROM ∧ TO rule,
already shipped in TKT-7I07IX).
- **Wiring per site**:
  - Request paths (data-entry actions, export_render, MCP lua_eval/lua_run):
`PolicyReader` + visible tracer — the script reads the caller's redacted view.
  - Scheduler: keeps its genuine `system:*` principal + `triggered_by`
(`scheduler.go:83-96`, confirmed present); receives `AllowAllReader` + raw
tracer **explicitly at the wiring site**. Note the ordering detail: the
principal is stamped on `taskCtx` while `LuaWriteDeps()` is evaluated separately
with no ctx — so the seam MUST resolve identity per-call from `r.callerCtx()`,
never bind ACL at construction time, or the scheduler principal is invisible to
it.
  - Validator: AllowAll for rule-body lookups; document why (finding 4).
  - CLI + docs runtime: AllowAll (operator trust boundary, RR-17DMC precedent).
- Arch-lint: only if option (b) is chosen — `lua.mayDependOn` currently lists
ai/filter/metamodel/principal/search/secrets/store/tracer and would need
`visibility` added.
- NopACL byte-parity test: without acl.yaml every Lua binding's output is
byte-identical to today.
- **Prove the LLM-job path end-to-end**: one scheduled job wired with a
role-scoped `PolicyReader` under a `system:<job>` principal (test or example
config) asserting: hidden field absent from `get_entity`/`search` results inside
the script; hidden entity invisible to `list_entities`/`trace_from`; audit log
attributes the job honestly.

## Carried in from PR 2 (TKT-L9Q669)

- **Closes the export_render residual.** PR 2 threaded the request principal
into `ExecuteDocument`, but the override script's OWN Lua reads stay unredacted
until this ticket lands. `docs/transforms.md` §Access control documents that
residual and must be updated here to say it is closed.
- **Re-verify the document-render singleflight key** (RR-2QSGLU). PR 2 added the
principal to the key defensively, *because* this ticket makes script reads
principal-scoped — at which point a wrong key becomes a cross-principal data
leak rather than an attribution bug. Add a test that two principals rendering
the same document concurrently do not share a result.
- `visibility.Redact` (exported in PR 2) is available for already-gated,
already-loaded entities where a type-claimed `Get` doesn't fit.

## Non-goals

Per-job role authoring UX (follow-up). MCP tool reads (`show_entity` etc —
separate follow-up closing RES-H5AB7S's gap). Egress controls (TKT-Z1OP7R — the
other half of the LLM-job safety envelope). Write-back/derivation provenance.
Read-side ACL bypass for scripts (the write side has `ElevatedManager` /
`rela.bypass_acl` per TKT-D8T148; the read side has no counterpart and this
ticket does not add one — the AllowAll reader is a wiring choice, not a
script-invokable escape hatch).

## Risks / notes

- **Data destruction risk if finding 2 is ignored** — see above; the
read-before-write in `luaUpdateEntity` must not be redacted.
- Behavior change: data-entry-invoked scripts now see the caller's redacted
view; scripts assuming full reads must run under an allow-all wiring or a
suitable role. Release-note it.
- The write path (`Mutator`/entitymanager) is otherwise untouched.
- Branch from `develop` **after #1188 merges** — that PR has `lua`-adjacent
wiring (document.go/executor.go) in flight; stacking risks a conflicting
refactor of the same files.
