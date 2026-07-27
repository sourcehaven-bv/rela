<!-- This file is auto-generated from docs-project/entities/. Do not edit directly. -->

# ACL: Security Hardening

This guide covers the security properties an operator running
rela with an `acl.yaml` needs to understand. Read [GUIDE-acl-overview]
first; this assumes you know the resolver vocabulary.

## Hardening the membership relation

rela's v1 ACL confers group roles by walking the **membership
relation** from the principal — the relation type named by
`membership_relation:` in `acl.yaml`, default `member-of`. By default
that relation is a regular relation type — there is no built-in
restriction on who can create one. **If you use groups in
`assignments`, you must gate writes to the membership relation**, or
any user who can create such a relation can grant themselves any role.

The simplest hardening is to require a `member-of:create` permission
on the relation and grant it only to administrative roles:

```yaml
role_relations:
  member-of:
    requires_permission: member-of:create

roles:
  admin:
    permissions: [member-of:create]
    create: ["*"]
    update: ["*"]
    delete: ["*"]
    read: ["*"]
```

With that in place, only principals who hold `member-of:create`
(directly or via inherited role) can add someone to a group.
Operators of single-user instances who don't use groups can ignore
this; the moment you add an `assignments` mapping for a group, this
is mandatory.

If you point `membership_relation:` at a domain-specific relation
(e.g. `heeft_rol` in a Dutch-language ISMS), the same hardening
applies to *that* relation — the `requires_permission` gate must name
the relation you actually configured:

```yaml
membership_relation: heeft_rol
role_relations:
  heeft_rol:
    requires_permission: member-of:create
```

The `rela acl audit` linter (below) flags an un-gated membership
relation — default or configured — as a high-severity finding, so run
it after editing `acl.yaml`.

The companion section in `docs/server-security.md` carries the same
guidance with more context on the broader threat model.

## Auditing your policy with `rela acl audit`

To keep your policy correct and hardened, run `rela acl audit` after
every change to `acl.yaml`. It catches the mistakes that don't fail
boot: a typo'd entity type that silently grants nothing, an un-gated
membership relation that opens a self-promotion path, a write grant on
the `everyone` role that hands capabilities to anonymous callers.
Boot-time validation (`Policy.Validate`) deliberately stays a narrow
*structural* gate — it rejects malformed policies but does not hunt for
these foot-guns; the audit is the tool that does.

The audit reports severity-ranked findings across two tiers:

- **Pure-policy** — escalation foot-guns (un-gated membership relation
  or role-relation conferring a privileged role; a privileged
  `everyone` role) and dead/inert config (assignments or `confers`
  naming an undeclared role, a `requires_permission` no role grants,
  dead permissions, wildcard write sprawl).
- **Metamodel cross-check** — grants, `membership_relation`,
  `role_relations`, `inherit_roles_through`, `user_entity_type`, field,
  and option references that the schema doesn't declare (silent drift).

```console
$ rela acl audit
⚠ ACL audit: 2 finding(s)
  [critical] role "everyone" ... grants write or permissions ...
      fix: remove write/permission grants from the everyone role ...
  [high] membership relation "member-of" confers group roles ... not gated ...
      fix: add role_relations.member-of.requires_permission ...
```

The audit is **advisory** — with no flag it prints findings and always
exits zero. For CI, gate the exit code on a severity threshold with
`--fail-on`:

```bash
rela acl audit --fail-on=high     # non-zero if any critical/high finding
rela acl audit --fail-on=medium   # also fail on medium
rela acl audit --fail-on=any      # fail on any finding at all
rela acl audit --exit-code        # alias for --fail-on=high
```

A finding **below** the threshold never changes the exit code — so
`--fail-on=high` lets a medium "wildcard write" nudge through without
breaking the build, while `--fail-on=any` is the strictest gate.
Findings are always printed regardless of the threshold.

**For a production deployment, gate CI on `--fail-on=any`** and keep
the policy clean — the medium/low findings (wildcard sprawl, dead
permissions, drift) are exactly the slow rot you want to catch before
it hides a real hole. Loosen to `--fail-on=high` only if you have a
deliberate, reviewed reason to tolerate the lower-severity findings.

Use `-o json` for machine-readable output. A clean, well-gated policy
reports no findings and exits zero.

## Resolving the principal by property (`principal_property`)

When `user_entity_type` + `principal_property` are set, the resolver
maps the raw authenticated identifier to a user entity by looking it up
against that property (see GUIDE-acl-overview). Because the property is
used as an **identity key**, a duplicate value is a security ambiguity,
not just a data-quality issue — two `persoon` records with the same
`email` would make "who is this principal?" unanswerable.

Two layers keep that from happening:

1. **The property must be declared `unique: true`** on the
   `user_entity_type` in the metamodel. Boot fails if
   `principal_property` names a property that does not exist or is not
   unique. `unique: true` is a general metamodel constraint: no two
   entities of the same type may carry the same non-empty value for that
   property. It is enforced at write time — a create or update that would
   introduce a duplicate is rejected as a validation error (422). Empty
   values are exempt (a property is unique among the entities that set
   it).

2. **The resolver refuses to guess through a duplicate.** If, despite the
   constraint, more than one entity matches (e.g. data predating the
   constraint), the resolver keeps the raw principal, logs a warning, and
   grants only what the raw string is assigned — it never silently picks
   one of the duplicates.

**Enforcement rigor by backend.** The write-time uniqueness check is
exact on the single-writer backends (fsstore, memstore). On PostgreSQL,
concurrent writers leave a narrow time-of-check/time-of-use window
between the check and the durable write. Operators who need race-free
enforcement add a partial unique index — which is also the recommended
performance index for the lookup itself:

```sql
CREATE UNIQUE INDEX persoon_email_unique_idx
  ON entities ((properties->>'email'))
  WHERE type = 'persoon';
```

With the index in place a colliding write fails atomically in the
database and surfaces as the same conflict the write path already
reports.

**The write-time check is not atomic.** It reads the existing entities
of the type, then writes — two separate operations with no lock held
across them. Under concurrent writers (the data-entry server runs one
goroutine per request) two racing writes with the same value can both
pass the check and both commit, on *any* backend, not just PostgreSQL.
The window is small, and the resolver's multi-match fallback (keep-raw,
above) is the runtime backstop for the identity-key case — but the only
*race-free* enforcement is the store-level unique index. If you rely on
uniqueness for correctness rather than as a data-quality guard, add the
index.

**Two operator notes for `principal_property`:**

- **Case sensitivity.** The lookup and the uniqueness check are exact
  byte comparisons. If your auth proxy emits `JV@x.com` but the persoon
  stores `jv@x.com`, the lookup finds no match and the request silently
  falls back to the raw principal (losing grants, never gaining them).
  Normalise the property value on write (or configure the proxy to emit
  a canonical form) — rela does not case-fold.

- **Resolution is data-entry-only.** The `principal_property` lookup is
  wired into the `/api/` data-entry path only. Writes via the CLI, MCP,
  scheduler, or desktop entry points authorize against the raw stamped
  principal and are NOT resolved to a `persoon`. A grant keyed on the
  resolved entity ID therefore applies to data-entry writes but not to
  the same person's CLI/MCP writes (which fall back to whatever the raw
  identifier is assigned — typically `everyone` only). This is
  fail-closed. Don't assume one `acl.yaml` grants the same authority on
  every transport.

## Roles from a verified assertion (`asserted_role_assignments`)

`asserted_role_assignments` maps a `roles` claim from a signed identity
assertion to a policy role (see GUIDE-acl-overview for the mechanics).
Three properties are load-bearing.

**The trust boundary is the signature, and it is enforced structurally.**
`internal/acl` verifies nothing — it trusts the `Principal` absolutely.
A role reaching it from an unverified source would therefore be a
complete authorization bypass, not a degradation. The assertion claims
live in *unexported* fields on `principal.Principal`, populated only by
`principal.Verified`, which only the JWT resolver calls after signature
verification. No composite literal anywhere in the tree can forge a
role; the compiler enforces this rather than a reviewer remembering to.

Consequence for operators: **never introduce a header that is trusted as
a role source.** Trusting `X-Asserted-Roles` the way `X-Forwarded-User`
is trusted under `--principal-header` would hand role selection to
anyone who can reach the server. The header path deliberately cannot
carry roles at all.

**The claim → role indirection is the second control.** A claim value
never becomes a role name; it only ever selects an entry the operator
authored. An attacker who influences role names in the IdP still cannot
name a rela role the operator has not mapped. Keep the mapping minimal
and review it like any other grant — `rela acl audit` flags a claim that
maps to a privileged role.

**Revocation lags by one assertion lifetime.** rela re-checks nothing
between requests: an assertion stays valid until its `exp`, so a role
revoked in the IdP keeps working in rela until the token expires. With
Pratique that window is **up to 9 minutes** (`AssertionTTL`). rela has
no revocation channel — there is nothing to "push" a revocation to. If
your incident response depends on cutting access faster than the token
lifetime, you must restart or otherwise intervene at the proxy; do not
assume revoking in the IdP takes effect immediately downstream.

### `org_id` is recorded, not enforced

A verified assertion's `org_id` / `org_slug` are stamped on the
principal and appear in the audit log. **Nothing in `internal/acl`
evaluates them.**

Read that literally before designing around it: a principal in org A
holding a role sees **every entity that role grants, in every org**.
There is no tenant isolation. The org in your audit log records which
tenant a request *came from*; it does not constrain what that request
could reach.

This is a deliberate scope decision, not an oversight — ACL evaluation
is additive (any role granting the verb allows it, and no rule can
subtract), so a cross-cutting "deny unless org matches" needs its own
design rather than a field check bolted onto the resolver. A test
(`TestAssertedRoles_OrgIsNotEvaluated`) pins the absence of enforcement
so it cannot be mistaken for an omission, and fails the day a partial
org check is added.

If you need per-tenant separation today, model it in the graph:
containment edges plus `inherit_roles_through` scope roles to a subtree
positively, and read-filtering follows for free. That works only if no
role grants a broad wildcard read — a global `read: ["*"]` defeats any
org scoping, in this design or any other.

## Fail-loud on malformed `acl.yaml`

A malformed `acl.yaml` fails boot. This is intentional: silently
degrading to NopACL (allow-all) on a typo would invert the operator's
intent — they wrote a policy specifically *to* restrict access. The
operator sees a clear error referencing the parse failure and the
file path; they fix the file and restart.

A genuinely absent `acl.yaml` is different: no file means "no access
control intended," and the server boots with `acl.NopACL{}`.
`rela-server` will warn at startup if you also bind non-loopback in
this configuration (no auth + no ACL + reachable from the LAN is
almost never what you want).

## Why `nil Subject` panics

`AuthorizeWrite` panics when its WriteRequest carries a nil Subject.
This is not a security feature per se — it's a programmer-error
guard. A nil Subject in a request means a call site forgot to
construct one, and the safe-looking alternatives (silently allow,
silently deny) both lose information:

- Silently allow → bypasses the ACL entirely at a single buggy site.
- Silently deny → looks like a permission problem when it's a code
  problem; the operator wastes time tweaking the policy.

The panic surfaces the bug at the call site, where it can be fixed,
on the first request that hits it. The dataentry call sites are
exhaustively tested for Subject population so this never reaches
production traffic.

## SSE event stream: per-type gating + audit isolation

The data-entry SSE event stream (`/api/events`, `/api/v1/_events`) is a
**cache-invalidation signal**, not an event log: it tells a connected
browser "entities of type T changed, re-fetch your active views of T."
The re-fetch goes through the already-gated REST endpoints, so the feed
itself needs to carry almost nothing.

**Per-type ACL gating (TKT-POT9GQ).** Each entity create/update/delete
collapses to a single `entity:changed` frame carrying the entity
**type only** — no id. `handleSSE` gates the type per-connection with
`readGate.ReadQuery(type)`: a connection receives the frame only if its
principal's verdict for that type is not `DenyAll` (AllowAll or a
relation-scoped Query both deliver). The type verdict is resolved once
per connection and cached. A relation write (member-of / role-relation /
containment edge — the types that can change a principal's read scope)
re-derives a **fresh** read gate for the connection: a new `acl.Request`
whose member-of closure is walked against the current graph, so a
principal who gains or loses a group membership mid-connection starts or
stops receiving a type's nudges without reconnecting. (The connection's
original request memoizes its membership closure for its lifetime; the
re-derive is what keeps the verdict honest — without it, only the
per-entity inbound query would refresh, not the principal's own group
membership.) Both the re-derive and the nudges are coalesced into one
flush window, so a bulk relation import triggers one membership re-walk
per connection, not one per edge.

Why per-type rather than per-id: the wire never carries an entity id,
so the feed cannot be a per-entity existence oracle for entities a
principal cannot read. The only residual is **per-type activity
timing** for types the principal can already read — which they could
equally infer by polling the gated list endpoint's count. A
fully-denied principal receives nothing for that type. (The richer
per-id / opaque-cache-id and snapshot-versioned-ACL designs were
considered and rejected as over-engineered for a staleness signal —
see the TKT-POT9GQ design record and IDEA-CQMKMD.)

**Audit isolation.** The stream deliberately does NOT carry audit
records, principal identity, or attribution chains. A denied write
produces a `denied-write` audit row server-side (with full attribution)
and **zero** events on the SSE wire. This matters because SSE is a
fan-out channel — putting audit attribution on it would leak the
principal-to-entity topology to anyone connected. With per-type gating
the feed now carries even less (just a type a connection may read).

A regression test in `internal/dataentry/sse_audit_isolation_test.go`
pins the audit invariant; `internal/dataentry/sse_acl_test.go` pins the
per-type gating. Future work that adds new SSE event types must
preserve both; a new audit-aware channel needs a separate broker with
its own per-subscriber gating.

## Deny response shape

A 403 from a denied write carries `rule_kind`, `rule_id`, and a
human-readable `reason`. It does NOT carry the principal's
attribution chain. This is by design:

- The wire body is what the *requester* sees. Telling them which
  groups they're in and via what edges would reveal organisational
  topology to a possibly-attacking client.
- The audit log is what an *operator* sees. It has the full chain.

If you're debugging "why was alice denied X?" — read the audit log
on the server, not the response body in the browser.

## Read-path gating

Read-side enforcement landed in two PRs with deliberately different
deny models:

- **Per-entity responses** (TKT-VQGN): deny is shaped exactly like
  not-found — a 404 indistinguishable from a nonexistent id.
- **Aggregates** (TKT-VMD8): lists, sidebar counts, and pagination
  metadata are shaped exactly like "the hidden entities don't exist" —
  filtered sets, filtered totals, no cardinality residue.

### Per-entity responses (TKT-VQGN)

ACL v1 gates every per-entity-response code path:

| Surface | Hidden-target behaviour |
|---|---|
| `GET /api/v1/<type>/<id>` | 404 with the same `not_found` body as a nonexistent id; no `ETag` header, no `If-None-Match` honoured |
| `PATCH/DELETE/POST` on `/api/v1/<type>/<id>` | 404 with the same `not_found` body, BEFORE body parse / `If-Match` / `IsLocked`; a malformed PATCH body on a hidden id returns 404, not 400 |
| `?include=*` and `?include=<type>` neighbours on any GET | hidden neighbours silently omitted from the `included` map; one batched `store.GraphQuery` per neighbour-type (not per neighbour) |

The deny shape is "indistinguishable from not-found." An attacker who
can probe URLs sees only 404 for every hidden entity, regardless of
verb. The 403 path is reserved for **visible-but-write-denied** — a
principal who can read the type but not write to that specific
record. That 403 still carries `rule_kind` / `rule_id` for
operator debugging.

### Invariants downstream maintainers MUST preserve

- **No `ETag` on deny.** Suppressing it is what closes the
  cross-principal cache poisoning surface. A future change that
  emits an `ETag` on the 404 path turns a denied principal's
  `If-None-Match: <alice-etag>` into a 304 — confirming existence.
- **All conditional-request headers short-circuit on deny.**
  `If-None-Match`, `If-Modified-Since`, `If-Match`,
  `If-Unmodified-Since`, `If-Range` MUST be consulted only AFTER
  the visibility probe passes. Today's handler emits only `ETag`,
  but the next maintainer to add `Last-Modified` needs to land the
  deny-side suppression in the same change.
- **The method dispatcher consults URL shape only.** Routing
  `GET/PATCH/DELETE/OPTIONS` for a path MUST NOT consult entity
  existence — the per-method handler is the gate. Otherwise an
  OPTIONS response shape becomes an existence oracle.
- **`?include=` uses the consumer-side `readGate`, batched per
  neighbour type.** A hub entity with 50 neighbours triggers ≤
  `O(distinct-types)` `GraphCount` calls, not 50. Future include
  surfaces (e.g. nested includes in a list response) MUST go through
  the same gate.

### Lists, sidebar counts, pagination (TKT-VMD8)

Anything that enumerates entities of a type returns only the visible
subset, with no leak surface revealing hidden cardinality. The list
pipeline (`scopedSortedEntities`, shared by `GET /api/v1/<plural>` and
`/api/v1/_position`) resolves the read scope **first**:

- **AllowAll** — a global role grants read on the type; the pre-ACL
  load path runs unchanged.
- **DenyAll** — no role grants any read; the pipeline returns empty
  **before** the search backend, structured filters, or sort run. A
  denied principal cannot probe backend latency (or induce index
  load) through `?q=`.
- **Query** — read is conferred via role-relations; a composed
  `store.GraphQuery` selects the visible subset, and search / filter /
  sort operate on that filtered slice only.

Every pagination surface derives from the post-filter total:
`data.length`, `meta.total`, `meta.has_more`, `X-Total-Count`,
`X-Page`, `X-Per-Page`, and the `Link` header rels — `rel="next"` is
absent when no *visible* next page exists, even when hidden pages
exist after it.

Sidebar counts go through the same gate, single-mode: there is no
"ACL off" code branch (a count path that only runs under ACL is a
count path that silently drifts). `listCount` / `kanbanCount` always
resolve the read scope, then `GraphCount` (no config filters) or
GraphQuery-then-filter (with config filters). Ordering is always
ACL → config filter → count, so a sidebar badge can never disagree
with the list it links to.

### Invariants downstream maintainers MUST preserve (aggregates)

- **The DenyAll short-circuit precedes the search backend.** A
  regression test pins the searcher at zero calls on the deny path.
  New work in the list pipeline must keep the scope resolution first.
- **Search runs after ACL, on the filtered slice.** This ordering is
  the contract the `/_search` gate (TKT-BA8BSX, below) generalized to
  mixed-type search; a mock-asserted test pins
  GraphQuery-before-search on the list pipeline.
- **No count from an unfiltered source.** Any new aggregate (badge,
  dashboard card, export count) must derive from the gated set, never
  from `Store.CountEntities` on a principal-reachable path.
- **Update/delete grants imply read grants; create does not.** The policy
  loader rejects a role with `update: [x]` (or `delete: [x]`) but no covering
  `read` entry at boot (structured error naming the role and type) — you must
  read a type to modify or remove it. **Create is exempt** (TKT-4LQMWP): a role
  may `create: [x]` with no read of `x`, reading back only what it authored via
  a role-conferring relation (e.g. `created-by`). This lets a "submitter" create
  a type yet see only its own entities of that type, with the normal Create
  button still shown (the affordance derives from the `create` grant). The
  invariant covers the `update:`/`delete:` lists; the affordance grant maps
  (`fields:` / `options:` / `relations:`) restrict surfaces within an authorized
  write and never confer writability by themselves, so they are intentionally
  outside the check.

### Global search (`/_search`, TKT-BA8BSX)

The search view runs through `search.VisibleSearcher` — a seam that
executes a free-text query restricted to a per-type visibility scope.
The dataentry layer resolves the principal's `ReadQuery` verdict for
every metamodel type into a scope map; the searcher guarantees no hit
outside that scope is ever yielded. The conformance suite
(`storetest.RunVisibleSearchTests`) pins the contract for every
implementation: any new searcher must pass it.

Key properties:

- **Scope lookup is fail-closed.** Exact type entry → reserved `"*"`
  wildcard entry → deny. With no ACL configured the gate supplies
  `{"*": allow-all}`, so entities whose type is absent from the
  metamodel stay searchable exactly as before ACL existed. Under a
  policy, no wildcard is emitted — an off-metamodel type (removed
  from `metamodel.yaml` while its files remain) is hidden from
  search rather than leaked.
- **The result limit applies after visibility.** `/_search` returns
  up to 1000 results; the bound counts *visible* hits. A
  pre-visibility cap would starve restricted principals — the top
  candidates may all be hidden while their own matches rank below.
  Both gate placement and limit placement are pinned by conformance
  cases on every backend.
- **Two implementations, one contract.** The fs/memory builds wrap
  the regular searcher in a generic filter (`search.NewVisible`,
  batched `MatchingIDs` probes — in-process, cheap). The postgres
  build composes visibility into the search SQL itself
  (`pgstore.SearchVisible`): hidden rows never leave the database,
  the `LIMIT` is post-visibility, and there is no hidden-row work to
  measure through timing.
- **Candidate-window caveat (bleve only).** The bleve backend caps
  candidate retrieval at 10000 hits; on the default build, "true
  top-1000 of the visible corpus" holds within that window. The
  linear and postgres backends have no such window. Related load
  note: a free-text query that also carries property filters defers
  all truncation until after the filters, so the generic path may
  load up to the candidate window's worth of entity bodies for one
  request — in-process on the local backends, but worth knowing
  when diagnosing search latency on very large projects.
- **Deny short-circuit.** An all-denied principal gets `data: []`
  without the search backend being invoked at all (no latency
  probe); a recording-searcher test pins zero calls — and exactly
  one call for a granted search.
- **Visible hits don't expose hidden neighbors.** Search results
  serialize without relation maps (`includeRelations=false`, pinned
  by test): a visible hit that relates to a hidden entity exposes no
  hidden ID or title through any field of its body.
- **Errors carry constant detail.** A visibility-evaluation failure
  maps to `500 acl_query_failed` / `504` / silent-on-cancel exactly
  like the list pipeline; a plain backend failure maps to
  `500 search_failed`. Raw backend error strings go to the server
  log only. (Previously these errors were silently swallowed into
  empty results.)

### Caching: per-principal responses

Under ACL, `/api/` responses differ per principal. Two layers keep
caches from leaking one principal's view to another:

- All `/api/` responses already carry
  `Cache-Control: no-cache, no-store, must-revalidate`.
- When `--principal-header` is configured, responses also carry
  `Vary: <that header>` — defense in depth for any cache layer that
  ignores `no-store`.

### Sidebar menu structure is principal-independent

The sidebar's *structure* (groups, labels, links) reveals metamodel
shape, not data shape, and is served identically to every principal —
only the *counts* are gated. Hiding whole menu entries per principal
is a possible future tightening, deliberately not done here: the
metamodel is not a secret (it's served by `/api/v1/_schema`), and a
divergent menu per principal complicates SPA caching for no
confidentiality gain today.

### Sidebar config-filter performance caveat

A sidebar list with `filters:` evaluates them in-memory after the ACL
GraphQuery — cost scales with the principal's visible-set size. For
visible sets beyond ~10k entities per type, prefer narrowing the nav
entry to a dedicated entity type, or file the follow-up that pushes
config filters down into GraphQuery.

## ACL fail-loud and middleware scope

The `attachACLRequest` middleware:

- **Wraps `/api/` paths only.** The SPA shell at `/` and static
  assets at `/static/` `/assets/` bypass it. A misconfigured
  principal stamper that throws `ErrUnstampedPrincipal` returns
  500 on `/api/v1/...` but lets the UI keep rendering. Otherwise
  operators would be locked out of the very surface they need to
  diagnose the misconfig from.
- **Fails loud inside `/api/`.** When ACL is configured and the
  principal stamper produces an unstamped principal, the middleware
  returns 500 with `acl_unstamped_principal` rather than silently
  proceeding with no `acl.Request` attached. Silent fall-through
  was a fail-open path — every read became AllowAll because the
  read handlers couldn't tell "no ACL" from "ACL but no
  principal."
- **Refuses script reads when the read gate cannot be built.** If an
  `acl.yaml` is configured but the visibility decorator fails to
  construct, script runtimes (data-entry actions, `export_render`,
  and the IdP webhook) receive a refusing reader rather than the raw
  store: every read returns `read gate unavailable; refusing to read
  ungated`, logged at ERROR at the wiring site. Degrading to ungated
  reads would be an unbounded silent disclosure on the unattended
  paths, where nobody is watching a warning. Note the asymmetry with
  traversals: `trace_from`, `trace_to` and `find_path` cannot report
  an error, so a refused traversal looks like an empty result to the
  script — correlate with the ERROR log by timestamp when a script
  reports an unexpectedly empty graph.

A project with **no** `acl.yaml` is unaffected: that is a deliberate
"no access control" configuration, not a fault, and scripts there read
the full graph exactly as they always have.

## Property-level redaction (`visible:`)

Entity-level filtering decides whether you see an entity at all. The
`visible:` grant is the orthogonal, finer cut: a field denied by
`visible:` is **omitted from the response `properties` map** even on an
entity you are allowed to read. This redaction is applied by the
data-entry serializer on **every** HTTP read shape — per-entity GET,
list rows, `?include=` peers, and `/_search` results — not just the
write form. When the hidden field is the entity's display property, the
`_title` falls back to the entity ID so the redacted value cannot leak
through the title. See [GUIDE-acl-overview] for how `visible:` grants
are written.

**Search cannot be a hidden-field oracle.** Redacting the response body
is not enough on its own: the full-text index still contains a hidden
property's text, so a query that matched *only* that property would
otherwise return the entity as a hit and confirm the value by its mere
presence (search a candidate postcode against a hidden address field →
guess oracle). `/_search` closes this at the `VisibleSearcher` seam: each
backend reports which fields a query matched (bleve with its own analyzer,
Postgres by re-matching the candidate rows per field, the linear backend
by substring), and a hit is **dropped** when every field it matched is one
the principal may not see. A hit that also matched a *visible* field — or
the id or content, which are never property-gated — still surfaces, with
the hidden property redacted from its body. The property-level conformance
suite (`storetest.RunVisibleFieldSearchTests`) pins this across all
backends.

## What still leaks (deferred)

- **`/api/v1/_position` per-id semantics** — `_position` is gated on
  both scope sources: list scopes share the gated list pipeline, and
  search scopes filter the search result through the read gate before
  computing ordinals, so totals and prev/next always come from the
  principal's visible subset. What remains scoped to the follow-up
  ticket: the per-id gate on the *requested* id and the
  neighbor-disclosure analysis (a visible neighbor's id confirms a
  visible entity, but gap analysis around hidden entities needs its
  own treatment).
- **MCP transport** — tracked as TKT-G3PPD. MCP read tools
  (`show_entity`, `list_entities`, `search_entities`, trace) apply
  neither the entity-level read gate nor `visible:` redaction; they
  return full entity bodies. The MCP server is local-only (stdio), so
  this is an accepted gap at this stage.

For threat-modelling purposes today: per-entity GET, write, include,
list, sidebar, pagination, global-search, and the SSE event stream are
all read-gated (the SSE feed per-type, see above); `visible:` redaction
applies to every data-entry HTTP read body; and `/_search` cannot be
used as a hidden-field oracle. The remaining read-side gap is the MCP
transport (TKT-G3PPD); within the data-entry server every read channel a
browser can reach is tight.

## Version history read gating (`history:read`)

The PostgreSQL build's [version history](postgres-backend.md#version-history-time-machine)
is read-gated on the same principles as everything else:

- **Live-entity history** (`GET /api/v1/_history/<type>/<id>` and a specific
  version) is gated by the *same* per-entity read verdict as a plain GET of the
  entity. A principal who cannot read the entity gets a 404 indistinguishable
  from a nonexistent id — history is never a side channel for reading a hidden
  entity's content. Every returned snapshot is run through the same serializer
  redaction as a live read, so `visible:`-denied properties never appear in a
  historical version either.
- **Deleted-entity history** has no live entity to evaluate a per-entity verdict
  against (the conferring relations are gone), so it is gated on a global named
  permission, **`history:read`**. Grant it via a role's `permissions:` list,
  exactly like the delegate-X permissions:

  ```yaml
  roles:
    auditor:
      permissions: [history:read]
  ```

  Treat `history:read` as an **audit-everything-deleted super-permission**: it
  is global (not per-entity), so a holder can read the surviving history of
  *any* deleted entity of any type — including entities they never had live
  access to. Grant it only to trusted audit/compliance roles. A **non-holder**
  requesting a deleted entity's history gets the same 404 as a nonexistent id,
  so the permission boundary does not itself leak which deleted entities exist.

### Historical field redaction fails closed (`history:read-redacted`)

Field-level `visible:` redaction of a *historical* snapshot **fails closed**.
A `visible:` grant can be conditional on the subject entity's own graph state —
`visible: has_relation(entity, 'reviewed-by')` or
`count_relations(entity, 'blocks') > 0`. Evaluating such a grant needs the
entity's edges *as they were when the version was captured*, but the live store
only holds its edges *now*: for a deleted entity there are none, and for a live
entity they may have drifted. Trusting the live store would let a grant that was
DENIED at write time flip OPEN and leak a field that was hidden when the version
was written.

So historical redaction does **not** trust the live store for subject-world
predicates. When serializing a snapshot, `has_relation` / `count_relations`
resolve as *no edges*, so any grant that needs them evaluates false and the
field is **hidden**. The rule, stated as an invariant:

> A historical version shows a field only if today's `visible:` policy
> affirmatively grants it against inputs that can be safely evaluated. Every
> grant that would need untrusted subject-world state is hidden. Reader-side
> inputs (`current_user`, the roles the reader holds) stay live — so redaction
> is still per-reader, and a reader who gains a role gains historical
> visibility.

This is deliberately conservative: it can **over-redact** (hide a field a reader
could legitimately have seen) but never **under-redact** (leak). A grant that
references a property the frozen record no longer carries (a since-renamed
field) likewise binds Nil and hides the field — a drifted schema over-redacts,
never leaks.

To see the fully un-redacted frozen snapshot, a role must hold the global
**`history:read-redacted`** permission — the field-grained sibling of
`history:read`:

```yaml
roles:
  auditor:
    permissions: [history:read, history:read-redacted]
```

Its semantics are **OVERRIDE**, all-or-nothing like `history:read`: a holder
sees *every* frozen field of a historical snapshot, bypassing field redaction
entirely (the snapshot already contains every field; the reveal just skips the
strip). Grant it only to trusted audit/compliance roles — it exposes fields the
live `visible:` policy would redact. A non-holder always sees the fail-closed
redaction described above.

## Command execution gating (`command:*`)

Data-entry [commands](data-entry.md#commands) execute arbitrary shell via
`sh -c`. They have no entity subject to evaluate — a `context: global` command
acts on the project, not a row — so like `history:read` they are gated on
**global named permissions**, declared per command and granted via a role's
`permissions:` list:

```yaml
# data-entry.yaml
commands:
  nightly-export:
    context: global
    script: "./scripts/export.sh"
    permission: "command:nightly-export"
```

```yaml
# acl.yaml
roles:
  operator:
    permissions: [command:nightly-export]
```

The permission name is arbitrary; `command:<id>` is a readable convention, not
a requirement.

**Authorization is bimodal, by design:**

- **No `acl.yaml` configured** → every command runs. A project that has not
  opted into access control behaves exactly as it did before this gating
  existed.
- **`acl.yaml` present** → a command runs only if its `permission:` is set and
  the principal holds it. A command with no `permission:` is **denied**: under a
  configured policy, an ungoverned shell-exec surface is treated as a
  misconfiguration, not as an implicit grant.
- **`--read-only`** → every command is denied, whatever the policy says.
  Command execution is not an `acl.WriteRequest`, so `ReadOnlyACL` cannot deny
  it through `AuthorizeWrite`; the command handler checks read-only mode
  directly. Without that check, `--read-only` would not stop a command from
  mutating the project.

Commands the principal cannot execute are filtered out of the command-resolution
API, so their buttons do not render. Treat that as a UI affordance only — the
exec endpoint re-authorizes and 403s independently, and the 403 body never
names the required permission or any policy content.

A running command can be cancelled only by the principal that started it; a
cancel request naming another principal's execution gets the same 404 as an
unknown one, so cancellation cannot be used to probe what else is running.

### What a command permission actually confers

**Command payloads are not read-gate scoped, in any context.** A command's
stdin JSON is assembled directly from the store, without the per-entity
`PermitsRead` verdicts that gate an ordinary API read. Granting
`command:<something>` therefore confers **read access to whatever that
command's context assembles**, not merely the right to run a script:

| context | what the script receives | scoped by |
| ------- | ------------------------ | --------- |
| `entity` | the entity at the caller-supplied `entity_id`, plus **every** incident relation | nothing — any id in the store |
| `list` | every entity in the caller-supplied `list_id`, post-filter | nothing — any configured list |
| `global` | project paths only | n/a |
| `view` | the entry entity plus the entire traversal closure | *not grantable — see below* |

Note the `entity_id` and `list_id` come from the request, not from whatever
page the user was on: `available_on` restricts where a **button appears**, not
what a command may be pointed at. Treat a command grant as "may read every
entity of this shape", and scope grants accordingly.

**`context: view` cannot be granted per-command at all.** View commands are
denied outright under any configured `acl.yaml`; `permission:` on a view
command is inert and warns at config load. The difference from the contexts
above is degree, not kind: a view's traversal closure is unbounded by the
config an operator can read, so the blast radius of the grant is not knowable
in advance. Deferred until the traversal is read-gate scoped.

Restore (`POST /api/v1/_history/<type>/<id>/<version>/restore`) is a **write**,
not a read: it is authorized as an ordinary update (or create, if the entity was
deleted), runs the per-field write gate on exactly the fields that change (so it
cannot set or clear a field the principal lacks write access to), and is audited
and re-versioned like any edit.

Not point-in-time: history read uses the *current* ACL, not the ACL as-of each
version. Reading a live entity's history exposes its **entire** history from
creation — including versions written before the principal gained access, and
content later edited out. If you redact content for compliance, understand that
older versions still hold it; a dedicated version-purge primitive is the
intended follow-up for hard removal.

**Field-visibility caveat for *conditional* `visible:` grants.** A historical
snapshot is field-redacted against the **current** ACL context (the live
relations and roles), not the context as-of the version. For an unconditional
per-type `visible:` grant this is exactly correct. But if a grant is
*conditioned* — on a relation (`visible: has_edge(...)`) or on another property
value — the verdict computed for a snapshot can differ from the entity's verdict
at write time, and for a **deleted** entity (whose conferring relations/roles no
longer resolve) it can under-redact. Until the visibility verdict is frozen at
capture time (a tracked follow-up), a policy that hides fields via *conditional*
grants should not rely on history-read redaction for those fields. Unconditional
per-type `visible:` grants are unaffected.

### Relation history: dual-endpoint gating

Relation version history (the PostgreSQL build versions relation properties and
body too) is read-gated on **both** endpoints: a caller must be able to read the
`from` AND the `to` entity. Gating on the `from` alone would make the `to`
endpoint an existence/content oracle — a principal allowed to read `from` but
denied `to` could enumerate `from`'s outgoing relation histories and learn about
the hidden `to` endpoint. A deleted relation (endpoints gone) uses the same global
`history:read`; a non-holder gets the same 404 as a nonexistent relation.

Note that relations have **no field-level (`visible:`) redaction** anywhere today
— a live relation GET returns its properties in full — so relation history
exposes exactly what a live relation read exposes, no more. A relation
field-redaction path is a separate follow-up; the dual-endpoint gate is what
bounds relation-history visibility today.

## Where to read next

- [GUIDE-acl-overview] — operator's overview of the resolver.
- [CON-authorization] — vocabulary and core concept.
- `docs/server-security.md` — the broader `rela-server` threat model
  (CSRF, DNS rebinding, loopback binding, command allowlist).
