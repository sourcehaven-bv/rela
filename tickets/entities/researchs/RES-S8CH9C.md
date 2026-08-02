---
id: RES-S8CH9C
type: research
title: 'Multi-app rela-server: serving several projects (ticketing, ISMS, …) from one process with a switcher'
summary: 'Multi-app rela-server as a horizon, reached by independently-justified refactorings. Storage: schema-per-project (DSN is the seam preserving DB-per-tenant and (tenant,project)->shard). Cross-project is a federation feature that must work ACROSS INSTALLS, so it is not a storage-layout tradeoff. Tenancy: control-plane-as-rela-project + wire the existing VerifyAssertion claims. R1 (schema-qualify advisory locks) is a latent bug to fix first.'
status: done
---

## Problem

One `rela-server` process serves exactly one project. Running a ticketing system
*and* an ISMS means two processes, two ports, two URLs, no shared chrome. The
target is one server, one URL, a pulldown to switch.

**This document is written as a horizon, not a project plan.** The intent is to
drive smaller refactorings that each stand on their own merit while making the
eventual setup easier. Nothing here should be built as a big-bang feature.

## The horizon

One process (or a fleet), N projects, a switcher listing what the signed-in
principal's org may open. Storage sharded as far as demand requires, with the
resident set of live projects bounded independently of total tenant count.

**The load-bearing invariant that gets us there:**

> As long as a single lookup — `(tenant, project) → DSN` — is the *only* thing
> that knows where a project's data lives, tenant count is an operations
> question, not an architecture one.

That lookup resolves to a `search_path` on one instance today, a different host
later, and a mix per deployment. `pgstore.Open(ctx, dsn)` already takes a DSN
and nothing else — it cannot tell the difference. **The project registry and the
shard map are the same object.** Keeping that true is the design constraint
worth enforcing: the moment anything else hardcodes a DSN or assumes one
instance, sharding stops being a config change.

The corollary for process cost: at high tenant counts you never hold N live
`Services` (each = bleve index, backfilled entities, Lua caches, watcher, SSE
broker, scheduler goroutine). You lazy-load with LRU eviction via
`Services.Close()` — the mechanism `rela-desktop` already exercises on every
project switch. Sharding bounds storage; eviction bounds process memory. Same
insight, two resources.

## Where the codebase already is

**The plumbing is largely done; the isolation is not.** The
workspace-decomposition arc removed the `internal/workspace` singleton;
`appbuild.go:1002` says so outright — "workspace was a process-singleton;
appbuild is constructed per project on a long-running desktop".

- `appbuild.Services`: per-project bundle, 22 accessors, all instance fields,
zero package-level state, `sync.Once`-guarded `Close()` (`appbuild.go:982`).
- `dataentry.NewApp(...)` takes the bundle explicitly; `project.Context` is a
pure value type.
- **`rela-desktop` already runs N projects in one long-lived process**
(`cmd/rela-desktop/main.go:126-221`): build new `Services` + `App` + router,
cancel old scheduler, swap under RWMutex, `Close()` previous *outside* the lock.
Serial not concurrent, but the lifecycle discipline is exactly right.
- Already per-instance, no collision: `acl.yaml` + `Declarative`
(`appbuild.go:542`); `acl.Request` amortized per-HTTP-request
(`router.go:196-293`); bleve per-`Services`, in-memory (`appbuild_fs.go:47`);
audit sinks per project dir (`appbuild.go:689`); scheduler + watchers per
`Services`; NOTIFY channels already schema-scoped (`feed.go:65-74`).
- **`VerifyAssertion` already extracts `Subject`/`Email`/`OrgID`/`OrgSlug`/
`Roles`** (`verifier.go:186-204`), bounded against hostile input.
`principal.Verified(...)` already exists as the compile-enforced constructor
(`orgID` unexported, no setter). Both are built and tested — just unwired.
- The IdP webhook already carries `org_id` into a Lua action that provisions
entities (`webhook.go:175`). Org membership is *already* designed to land in a
rela graph.

## Cross-project is federation, not a storage-layout question

**Decided (2026-07-26): cross-project linking must work ACROSS INSTALLS**, not
merely across projects inside one database. That settles a question this
document previously got wrong.

An earlier draft listed "no cross-project queries" as a *downside* of
schema-per-project, implying a tenant column would have bought them via a SQL
join. That framing is void: a join only ever worked inside a single database, so
it was never a solution to federation across separate deployments.
Schema-per-project therefore forecloses **nothing that was actually available**,
and the storage choice below is unconstrained by this requirement.

Cross-project support is its own feature with its own design — stable
cross-install entity references, a resolution protocol, auth between installs,
and partial-availability semantics when a remote install is unreachable or a
reference dangles. Nearest prior art in-tree is the machine-to-machine sync API
(`/api/sync/`, `a.sync.registerSyncRoutes` at `router.go:95`). It should be
researched separately; it is explicitly NOT a byproduct of table layout, and it
must not be used to justify a storage decision in either direction.

## Storage: schema-per-project

Chosen over a tenant column on every table.

**Why the column option is worse than it looks.** `entities.id` is a bare `TEXT
COLLATE "C" PRIMARY KEY` (`0001_init.sql:31`). A tenant column makes that
composite and propagates: every PK/FK (`relations` `(from_id,rel_type,to_id)`,
`attachments`, `entity_versions`, `relation_versions`), ~20 indexes across 6
migrations including two GIN indexes on `search_text` (which don't take a
leading scalar efficiently), and every query in `pgstore` gains `AND project_id
= $n`. **Miss one and it's a silent cross-tenant leak with no compile-time
error.** Plus `rela_seq`/`version_seq` would interleave across tenants, muddying
the change-feed watermark.

Schema-per-project gets isolation from Postgres itself — wrong-schema access is
"relation does not exist", not a missing WHERE. Indexes, sequences, manual IDs
and collation-sensitive keyset pagination all keep working byte-for-byte. And
`testdb_test.go:84-115` already runs N migrated isolated schemas on one database
with per-schema pools: this is promoting the test harness, not new design.

**Honest downsides:** migrations run N times with mixed-version partial-failure
states; DDL sprawl across catalogs; project IDs become schema names so they are
a trust boundary needing strict validation (`^[a-z][a-z0-9_]{0,30}$`). Note that
in-database cross-project joins are NOT on this list — see the federation
section above.

**The connection ceiling — measured, and the reason it is NOT terminal.**
`pgstore.Open` (`open.go:24-66`) builds a pool per call with **no `MaxConns`
override** (pgx defaults to `max(4, numCPU)`) plus a **dedicated listener
connection outside the pool** (`open.go:52`), plus a sweep goroutine acquiring
from the pool. On a 16-core box that is ~17 connections/project; against a
typical `max_connections=100`, ~20 projects.

That ceiling is an artifact of **one pool per project**, not of schemas.
Schema-per-project preserves the option of one shared pool with `SET
search_path` per checkout, which raises it substantially. Not free — it fights
pgx's model and interacts badly with the sweep's session-scoped advisory lock
(`sweep.go` warns that issuing inserts via the pool would "silently void the
single-writer guarantee"). Not early work, but the option exists.

**DB-per-tenant** (considered, not chosen now): wins on blast radius (per-tenant
dump/restore, `DROP DATABASE` for GDPR erasure, per-tenant tuning and
credentials), and makes the advisory-lock and NOTIFY-channel qualification
redundant rather than load-bearing. But a pooled connection is **bound to one
database** — you cannot share a pool across databases, so it permanently
forecloses the one optimization that raises the ceiling. PgBouncer is the usual
escape and is a poor fit here: it conflicts with `LISTEN/NOTIFY` (needs session
pooling) and with session-scoped advisory locks. Right answer only when a
*compliance* requirement demands physical separation — a business input, not a
technical one.

**Sharding at high counts** — `(tenant, project) → DSN` returning a host instead
of a `search_path`. Composes with schema-per-project (shard to an instance, then
schema within it), so the connection ceiling becomes per-shard and scales
horizontally. Genuinely hard parts are not the routing: the map becomes critical
infrastructure needing caching/invalidation (and cannot live in a sharded rela
project without recursion — an argument for a small, boringly-hosted control
plane); **rebalancing** is a real project (stop writes, drain the change feed,
dump/restore, update map — subtle with the sweep's lock in flight); fleet
migrations gain partial-failure states; per-shard failure becomes partial outage
needing honest degradation in the switcher.

## Multi-tenancy: control-plane-as-rela-project

Nothing in identity or ACL can currently express "user X may access project A
but not B". `principal.Principal` (`principal.go:68-76`) has no project field,
and the org claim is explicitly non-enforcing — `principal.go:100-103`:
"ATTRIBUTION ONLY... a principal in org A holding a role sees every entity that
role grants, in EVERY org... Enforcement is deliberately deferred; see
**TKT-RP3X3Q**." Confirmed: `OrgID()` has one non-test caller outside its
package (`router.go:320`), zero ACL evaluation sites. The production gate calls
`VerifySubject` (`jwtgate.go:141`), **discarding org/roles before they reach
ctx**; the path that populated them (`JWTPrincipalResolver`) is deprecated and
unwired.

**Proposal: make the registry itself a rela project** (`_control`):

```yaml
entities:
  org:    { properties: [name, idp_org_id] }
  app:    { properties: [slug, project_path, dsn, description] }
  person: { properties: [email, idp_subject] }
relations:
  org--has-app-->app
  person--member-of-->org
  person--granted-->app     # optional per-user override
```

The data-entry UI becomes the admin console — no new CRUD screens; validation,
audit, versioning, MCP tools and Lua automations all apply. The IdP webhook
already fits (`membership.created` → Lua action → create person, link
`member-of`). ACL on the control plane is just its own `acl.yaml` — no second
authorization system.

**Do not hardcode org→principal.** `VerifyAssertion` already carries it;
hardcoding strands the second customer.

**Bootstrap** (the real objection — the control plane can't authorize access to
itself using itself): its own `acl.yaml` uses `asserted_role_assignments`
(`policy.go:100`) against an IdP claim, so the IdP is the root of trust and
there is no recursion. That path is already per-policy hence per-project, and is
dead in production *only* because the gate strips roles — so wiring
`VerifyAssertion` lights up an existing designed path rather than inventing one.

**Still Go work regardless of being metamodel-based:** wire the gate to
`VerifyAssertion` (respecting the deliberate identity-source exclusivity in
`main.go:199-231` — do not reintroduce an auth-downgrade path); project
selection *before* `attachACLRequest`; the switcher's list as a new
cross-project authorization decision with no existing home; a cache with
invalidation (note `acl.yaml` has no reload/watcher today).

**Keep per-project identity per-project.** `principal_property` resolution
(`router.go:306-323`) resolves against *the project's own graph*, so the same
email is a different `PERS-…` per project. Org membership lives in the control
plane; a globally-cached principal would be wrong.

## Refactorings that stand on their own merit

Ordered by independence. Each is justified without committing to multi-tenancy —
that is the point.

**R1 — Schema-qualify the advisory lock keys. ✅ DONE.** All three of pgstore's
advisory locks used unqualified compile-time constants, but PG advisory locks are
**database-wide**, so two schemas on one database contended on all three.
Severity varied sharply, which is why the fix arrived in two parts:

- **Version sweep** — the severe one. Non-blocking `pg_try_advisory_lock` plus a
  silent `return nil` on failure meant the losing schema's version capture was
  **dropped with no error, warning or metric**. Fixed independently by **#1217
  (TKT-SCXHUL)**, which surfaced it when parallel postgres e2e workers starved
  each other. Uses the two-key form
  `pg_try_advisory_lock(key, hashtext(current_schema()))`.
- **Migrate + write Tx** — blocking rather than lossy, so nothing forced them
  into view. Fixed under **BUG-CA3VY0**, adopting #1217's idiom rather than a
  competing mechanism.

Two things worth carrying forward from doing this:

1. **The low-severity instances of a defect outlive the high-severity one.** The
   sweep variant got fixed because it lost data visibly; the identical root cause
   in migrate and write survived that pass. When fixing a class of bug, grep for
   the class, not the symptom.
2. **A regression test for lock scoping is easy to write and useless.** Holding
   the *scoped* key and asserting the other schema proceeds passes even against
   the un-fixed code — Postgres treats one-key and two-key advisory locks as
   disjoint spaces, so the bare-key regression takes a lock nobody holds. The
   first draft of the tests did exactly this and passed against a deliberately
   broken build. Only a fail-before check caught it.

This was **not** a multi-tenancy prerequisite — it was a latent bug in the
current single-project design, since the conformance harness already runs many
schemas per database. It just happens to also be a precondition for
schema-per-project.

**R2 — Make the DSN an explicit parameter rather than an ambient env read.**
`appbuild.Discover` hardcodes `os.Getenv("RELA_DATABASE_URL")`
(`appbuild.go:698`); `Config.DatabaseURL` is *already* a per-`Config` field, so
`New` is fine and only `Discover` is ambient. **7 call sites** reach `Discover`
(`cmd/rela-server`, `cmd/rela-docs`, `internal/docscapture`,
`internal/cli/{mcp_wiring,flow,kong,validate}`) — broad but shallow. Merit on
its own: ambient env reads are the classic testability and
surprise-in-a-second-context problem. **This is the seam that preserves every
storage option** (schema / DB-per-tenant / shard) at zero cost now. Keep the
credential out of argv — config file or env, never a flag (`main.go:120-122`).

**R3 — Adopt desktop's teardown ordering in the server.** `rela-server`
deliberately never calls `svc.Close()` (`main.go:375-378`, "the daemon-lifetime
case"), while desktop does it properly. Merit on its own: makes the server's
lifecycle testable and correct under reload. Prerequisite for any registry that
adds/removes projects, and for LRU eviction later.

**R4 — Make per-project failure non-fatal.** `discoverProject`
(`main.go:144-153`) `os.Exit(1)`s. With N projects one bad project must not kill
the server. Merit on its own: better single-project diagnostics too.

**R5 — Route the frontend's API base through one place.** `api/client.ts:13`
axios `baseURL` is nearly the single chokepoint, but ~8 sites bypass it: raw
`fetch` in `api/settings.ts:74,82,95,103` and `api/theme.ts:24,40,59,92`;
builders in `api/attachments.ts:11`, `api/transforms.ts:35,53`; `EventSource`
(`useEvents.ts:95`); iframe src (`AppHostView.vue:21`); plus legacy non-`/v1`
paths (`HelpModal.vue:37`, `CommandModal.vue`). Merit on its own: consistent
interceptors, error handling, and testability.

**R6 — Wire the gate to `VerifyAssertion`.** Stops discarding verified claims.
Merit on its own: roles become available to `asserted_role_assignments`, which
is a documented, built, currently-dead feature. Respect the identity-exclusivity
rationale in `main.go:199-231`.

**R7 — Project-prefix-aware app CSP.** `appCSP(base)` is path-scoped
(`internal/dataentry/CLAUDE.md`); a prefix must flow into that base or the
iframe sandbox boundary silently loosens. **Security-load-bearing** — needs
review, not mechanical care. Only meaningful alongside prefix routing, so it is
the one item genuinely coupled to the feature.

**R8 (separate research, not a refactoring) — cross-install federation.** See
the federation section above. Independent of everything else here.

## Routing seam (when the time comes)

`App.NewRouter()` (`router.go:58`) builds a fresh `ServeMux` with absolute path
literals; `api_v1.go:133` strips a hardcoded `/api/v1/`; security middleware
assumes `/api/` at position 0 (`middleware_security.go:208,246,269`;
`router.go:167`). Cleanest: leave it untouched and wrap —
`outer.Handle("/p/{id}/", http.StripPrefix("/p/"+id, app.NewRouter()))` — so
internal literals and the middleware chain consistently see the stripped path.
Pin with the probe table in `router_walk_test.go`. Caveat: `spaHandler`
(`router.go:592-608`) serves an `index.html` referencing `/assets/*`, which 404s
under a prefix unless Vite `base` matches or assets mount unprefixed.

Frontend switch cost is **bimodal**: a hard-navigation switch
(`window.location.assign('/p/<id>/')`) skips *all* cache invalidation, route
changes and `/:project` threading — a reload wipes the three project-unaware
caches (Pinia `schema`/`entities`, the Colada cache at `queries/entities.ts:41`)
and module globals (`registerEntityPlurals`, the `eventSource` singleton) for
free, as chunk-reload recovery already does (`main.ts:58`). In-place switching
is several days more and carries two bug traps: the `loadPromise` in-flight
dedupe race (`schema.ts:59`) and that module-global plural registry.

There is **no frontend auth layer at all** — session is server-side same-origin
cookies, so a path prefix gets per-project cookie scoping free; a header scheme
would not. No top header bar exists; the sidebar header (`Sidebar.vue:146-156`)
already shows project logo + name and is the natural switcher home — but the
file is at 519 lines against a 500-line lint warning, so extract
`ProjectSwitcher.vue`.

## Recommendation

**Treat this as a horizon and bank R1–R6 opportunistically.** Each is
independently justified; together they mean the eventual feature is assembly
rather than surgery.

**R1 is done** (#1217 for the sweep lock, BUG-CA3VY0 for migrate + write) — it
was a latent bug in the current design, not a multi-tenancy concern, which is
exactly why it was the right thing to do first: it paid for itself regardless of
whether multi-app is ever built. **R2 (per-project DSN) is the next one to
bank** — it is the seam that keeps schema-per-project, DB-per-tenant and
`(tenant, project) → shard` all reachable, and its own justification (an ambient
env read is untestable and surprising in a second context) stands without any of
them.

When the feature is built, phase on trust model:
- **Trusted users, all projects visible** (one team running its own ticketing +
ISMS): prefix routing + hard-navigation switch + per-project DSN. Per-project
`acl.yaml` already works; the missing tenancy dimension is not needed. Days.
- **Users who must NOT see some projects**: the control plane + R6 + an
enforcement site. Until that exists, **separate processes behind a reverse proxy
is the honest answer** — process isolation provides the guarantee the in-process
model cannot yet make. A switcher listing projects a user cannot open is worse
than no switcher; one listing projects they *should not see* is an incident.

Accepted tradeoffs: logical not physical isolation in-process (a project's
expensive export shares CPU/memory); memory linear in *resident* projects (fine
for ~10, then lazy-load + LRU via `Services.Close()`). Cross-install federation
is deferred to its own design and does not constrain this one.
