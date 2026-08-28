---
id: TKT-P938T7
type: ticket
title: 'appbuild: separate the shared, tenant-independent base from the per-store assembly'
kind: refactor
priority: medium
effort: l
status: backlog
---

## Description

`appbuild` builds exactly one `Services` per call, and every call redoes the
tenant-independent work: parse `acl.yaml`, load and validate `metamodel.yaml`,
apply options. Make that front half a **reusable value** that can be built once
and assembled against several stores.

This is the spine of multi-tenant SaaS (RES-D54281) — one operator config, N
tenants — but the refactor is justified without it (see below).

**Do not build tenant resolution, routing, or an `org_id → DSN` map in this
ticket.** This ticket only makes the shared half reusable.

## The good news: the shape already exists

Construction is already split, and the split is already in the right place:

- **`prepare(cfg, opts) → *buildBase`** (`appbuild.go:739-765`) — validate config,
apply options, resolve/parse the ACL policy, load the metamodel. `buildBase`
(`appbuild.go:720-726`) holds `cfg`, `opts`, `acl`, `aclPolicy`, `meta`.
**Nothing store-bound.**
- **per-build `openBackend`** — opens the store + searcher.
- **`assemble(base, st, searcher, …) → *Services`** — wires everything that needs
the store.

`assemble`'s own doc already states the invariant: *"a recipe may CHOOSE and
ORDER backend steps, but build-agnostic wiring lives here and nowhere else."*
That is exactly the property this ticket preserves and extends.

So the work is **not** "move fields into a SharedConfig struct". It is: make
`buildBase` outlive one `Services` and be assembled N times.

## The finding that shapes the design

**The split is NOT along the `Services` field list.** `acl.Declarative` is built
by `resolveACL(base, st)` (`appbuild.go:786+`) because it needs a store-backed
`acl.Graph` for group expansion and containment inheritance — `prepare`'s doc
says so explicitly. So:

- the **ACL policy** (parsed `acl.yaml`) is shared
- the **ACL evaluator** is per-store

Same for `lua.ReadDeps`, which closes over the store. Anyone planning this as a
field-list partition will get it wrong.

Verified classification (commit e7003d74 / develop):

| Shared (no store dependency) | Per-store (built in `assemble`) |
|---|---|
| `fs`, `paths`, `meta`, `scriptEngine`, `aclPolicy` | `store`, `versions`, `searcher`, `visibleSearcher` |
| `templater` (`NewFSTemplater(fs, paths)`) | `tracer` (`tracer.New(st)`), `entityManager` |
| `cfgLoader` (`NewFSLoader(fs, root)`) | `validator` (`validator.New(st, meta, readDeps)`) |
| | `acl` + `aclDeclarative`, `searchCloser` |

`stateKV` is currently **neither** — it is rooted at the shared `CacheDir`
(`buildStateKV`, `appbuild.go:1010`), which is the collision TKT-VC27L3 moves
into PostgreSQL. Sequencing note below.

## Why this stands on its own

1. **Metamodel + ACL parsing is redundant work today.** `rela-desktop` already
builds a fresh `Services` per project switch (`cmd/rela-desktop/main.go`),
re-reading and re-validating config each time. A reusable base makes a switch
cheaper and makes "did the config change?" an answerable question.
2. **It makes the config/data boundary explicit and testable.** Right now nothing
stops a future collaborator that needs only the metamodel from taking the store
— the compiler permits it, and the coupling is discovered later.
3. **`Config` currently mixes concerns**: `FS`, `Paths`, `ScriptEngine`, `Audit`
are shared; `DatabaseURL` is per-store. TKT-1J5KEV (#1318) already began prying
that apart by making the DSN an argument rather than an ambient read.

## Scope

**In scope**

- Export (or otherwise make reusable) the `prepare` result so a caller can build
it once and assemble several `Services` from it.
- Keep `New`/`Discover` working exactly as today, implemented in terms of the new
path — one base, one assemble. No behaviour change for any current caller.
- A test that builds ONE base and assembles TWO `Services` against two different
stores, asserting the metamodel and ACL policy are parsed once and shared.
- Verify no shared value is mutated by assembly. `*metamodel.Metamodel` and
`*acl.Policy` are pointers handed to every tenant; if anything writes through
them, that is a cross-tenant bug and this ticket is where it must surface.

**Out of scope**

- Tenant resolution, `org_id → DSN`, routing, provisioning (RES-D54281).
- Per-tenant keyspacing of `state.KV` — TKT-VC27L3.
- Making `rela-server` actually hold N `Services`. This ticket makes it
*possible*; wiring it is separate.
- `dataentry.App` changes. `NewApp` already takes 11 explicit parameters with no
hidden globals, so it is already per-tenant constructible — confirm, don't
change.

## Load-bearing constraints

- **`assemble` must stay the single home for build-agnostic wiring.** The three
recipes (`appbuild_{fs,memory,postgres}.go`) exist because each CHOOSES backend
steps; the moment shared wiring is copy-pasted into a recipe they drift. This is
stated in `assemble`'s doc and in CLAUDE.md.
- **`Services.Close()` must stay per-store.** It currently closes only `store`
and `searchCloser` (`appbuild.go:985-1000`) — correct, and what makes per-tenant
eviction clean. It must NOT grow to close anything shared, or evicting one
tenant would tear down another's config.
- **Constructors reject nil required fields** (CLAUDE.md). A base that is
incomplete must fail at construction, not at first use.
- **No service locator.** The base is a purpose-specific bundle of *already
parsed config*, not a grab-bag to reach collaborators through.

## Sequencing

Land **TKT-VC27L3** (Postgres-backed `state.KV`) first, or at least decide it.
`stateKV` is the one field that is neither shared nor per-store, and shipping a
per-tenant assembly while it still roots at the shared `.rela/` would create a
live cross-tenant path rather than leave one latent — specifically the document
render cache, whose key carries no tenant component (`document.go:157,183`).

## Acceptance criteria

1. The tenant-independent half of construction is a value that can be built once
and used to assemble several `Services`.
2. `New` and `Discover` behave exactly as today for every existing caller; the
full suite passes on all three build tags with no test changes beyond the new
ones.
3. A test builds one base and assembles two `Services` against two stores; both
work independently, and the metamodel/ACL policy are parsed once.
4. Closing one assembled `Services` leaves the other fully functional — the
eviction property. Pin it with a test.
5. No shared value is mutated during assembly (see scope note).
6. `just arch-lint`, `just ci`, and `just test-postgres` pass.

## Notes

- `appbuild.Services` carries `//plimsoll:max-exported-methods=23` and is a
documented facade exception. This ticket should not grow that number; if it
does, that is a signal the base is leaking accessors it should not have.
- `appbuildtest.New` (`internal/appbuild/appbuildtest/fixture.go`) constructs
services for tests and roots its own state KV — keep it working, and prefer
routing it through the same new path so tests exercise production wiring.
