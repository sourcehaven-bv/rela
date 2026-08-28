---
id: RES-NH3P12
type: research
title: 'Metamodel-declared pointers (draft/published): storage model and ACL integration'
summary: Store pointers as a per-entity ref table (name → vseq) on the injected VersionService, declared per type in metamodel.yaml; make the pointer the ACL SUBJECT via a per-pointer read verdict (`type@pointer`) in acl.yaml, so the built-in `everyone` role reads `published` while unqualified grants cover the live row. Reject gating pointer reads on the live entity's read verdict — the history precedent — because it inverts the desired public/private relationship.
status: done
---

## Problem

We want an entity type to declare named **pointers** — e.g. `draft` and
`published` — each selecting one immutable version of an entity. Two questions:

1. **Storage.** Where do per-type pointer *declarations* live, and where
does per-entity pointer *state* (which version each pointer selects) live?
2. **ACL.** How does the read side grant `everyone` (incl.
unauthenticated) read of the `published` pointer while `draft` stays
visible/editable only to authors — without the two leaking into each other?

This matters now because the surrounding design (versions + pointers +
proposals) hinges on whether pointer-scoped ACL is expressible in the *existing*
declarative policy or needs a new axis. If it needs a new axis, that is the
expensive part and should be discovered before any surface design.

## Context

### Versioning storage

`internal/store/pgstore/migrations/0004_versions.sql:51` — postgres-only, and
already close to what pointers need:

```sql
CREATE TABLE entity_versions (
    entity_id      TEXT COLLATE "C" NOT NULL,
    vseq           BIGINT NOT NULL DEFAULT nextval('version_seq'),
    op             TEXT NOT NULL,          -- create|update|rename|delete|purge
    prev_id        TEXT COLLATE "C",
    type           TEXT NOT NULL,
    content        TEXT NOT NULL DEFAULT '',
    properties     JSONB NOT NULL DEFAULT '{}'::jsonb,
    content_hash   TEXT NOT NULL,
    schema_hash    TEXT COLLATE "C" NOT NULL REFERENCES schema_versions(hash),
    principal_user TEXT NOT NULL DEFAULT '',
    principal_tool TEXT NOT NULL DEFAULT '',
    triggered_by   TEXT NOT NULL DEFAULT '',
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (entity_id, vseq)
);
```

Four structural facts that constrain the design:

- **`vseq` is a stable global ordinal** — a pointer is just a named
`vseq`. The human-facing "version N" is a read-time `row_number()` over a
lineage, *deliberately not stored* (no app-side max+1 race, `0004:38-42`). **A
pointer must therefore reference `vseq`, never the ordinal.**
- **No FK from version rows to `entities`** (`0004:6-8`) — history
deliberately survives entity deletion. So a pointer table FK'ing
`entity_versions` inherits that independence for free.
- **Snapshots are full, not diffs.** `content`/`properties` are the whole
entity. No delta encoding anywhere.
- **Version rows carry no edges to other version rows.**
`relation_versions` explicitly omits `from_vseq`/`to_vseq`
(`0005_relation_versions.sql:41-45`): *"the endpoints' versions are resolved at
READ time (with the reader's ACL), never stored — storing them would leak a
TO-side oracle."* **This is load-bearing: there is no graph among version rows
to traverse.**

### Versions are Time Machine data — verified exhaustively

A full sweep for `entity_versions|relation_versions|schema_versions|
version_seq` across non-test, non-migration code hits **exactly four files**,
all inside `internal/store/pgstore`: `version.go`, `relation_version.go`,
`sweep.go`, `purge.go`. No other package names the tables. At the Go API level
the only callers of the history read methods are display and restore surfaces
(`internal/cli/history.go:41,72`, `restore.go:39`,
`internal/dataentry/history_handler.go:135,175`, `history_restore.go:39`, and
the relation twins).

Corroborating: no search/index integration (no tsvector/trgm/GIN on either
version table); no change-feed integration (`ManifestSince` unions
`entities`/`relations`/`deletions`, never a version table, and `version_seq` is
deliberately isolated from `rela_seq`). Version writes are invisible to sync
clients and observers.

**Bottom line: the version tables are an append-only, read-only-by-lineage side
log. Any feature wanting to traverse or analyze version rows would be the first
such consumer.**

### Versioning is an INJECTED SERVICE, not a type-asserted capability

Important correction to the older mental model. TKT-N0IKN9 extracted versioning
out of the store. `internal/store/pgstore/pgstore.go:77-79`:

> *Do NOT add version/history/purge methods back to `*Store`: they belong
> on `[VersionStore]`, and re-adding one here would resurrect the
> type-assert-off-the-store pattern this refactor removed. Consumers reach
> versioning through the injected `store.VersionService`, never by
> asserting a capability on the store.*

`store.VersionService` (`internal/store/store.go:768`) is the umbrella of
`HistoryReader`, `VersionWriter`, `RelationHistoryReader`,
`RelationVersionWriter`, `VersionPurger`, `RelationVersionPurger`. Satisfaction
is compile-time (`var _ store.HistoryReader = (*VersionStore)(nil)`,
`version_store.go:40-48`); consumers *narrow the umbrella by assignment* at the
call site (`var reader store.HistoryReader = svc.Versions`). Build-tag selected:
`versionServiceFor` returns a real service on postgres, **a genuinely nil
interface otherwise** (`appbuild/versionsweep_nosweep.go:19` — typed-nil would
defeat consumers' nil checks).

**Pointers therefore belong on `VersionService` (or a sibling injected service),
not on `store.Store`.**

Also relevant: the same file notes *"fsstore already gets content versioning
from git."* That is the portability escape hatch for pointers on non-postgres
builds — a pointer is conceptually a git ref.

### Read-side ACL

Two independent layers, both in `internal/visibility` per DEC-ZBI39P:

```go
// internal/visibility/visibility.go:58
type RowGate interface {
    PermitsRead(ctx context.Context, entityType, id string) (bool, error)
    PermitsReadMany(ctx context.Context, entityType string, ids []string) (map[string]bool, error)
}

// internal/visibility/visibility.go:70
type FieldRedactor interface {
    HiddenProperties(ctx context.Context, e *entity.Entity) map[string]struct{}
}
```

The row gate reduces to **one function**, `Request.readQuery`
(`internal/acl/readquery.go:27`), returning exactly one of:

```go
type ReadQueryResult struct {
    AllowAll bool
    DenyAll  bool
    Query    *store.GraphQuery
}
```

`PermitsRead`/`PermitsReadMany` are thin wrappers; the `Query` case pushes down
to `GraphQueryer.MatchingIDs` (SQL in pgstore).

**Two constraints shape everything below:**

1. **`Read` grants are unconditional.** `roleGrantsRead`
(`readquery.go:78`) is exact-or-`"*"` match on a flat `[]string`. Unlike
`FieldGrant` / `OptionGrant` / `RelationGrant`
(`internal/acl/policy.go:263-288`), each carrying a `When` predicate, **read has
no `When`**. Row read is all-or-nothing per type, refined only by
role-conferring relations.
2. **`store.GraphQuery` is relation-shaped only**
(`internal/store/graphquery.go:20`) — `EntityType`, `HasInbound`, `HasOutbound`.
**No property predicate.** "Rows whose `published` pointer is set" cannot be
pushed down today.

### ACL policy lives in `acl.yaml`, and there is no `visible:` in the metamodel

A correction worth stating plainly: **`visible:` is a `RoleDef` field in
`acl.yaml`** (`internal/acl/policy.go:229`), not a metamodel property attribute.
`metamodel.PropertyDef` (`internal/metamodel/types.go:268-320`) has no
visibility field; the metamodel is consulted only as the closed-world *field
universe* (`affordances/resolver.go:418`). (The `visible_when` in
`internal/dataentryconfig` is UI conditional display, unrelated.)

`acl.yaml` absence = allow-all (`appbuild.go:547`). Loading is tolerant: unknown
top-level keys warn and are ignored.

### `everyone` already is the public mechanism

`acl.EveryoneRole = "everyone"` (`internal/acl/policy.go:34`) is held implicitly
by every principal, authenticated or not, appended in both the write path and
the affordances resolver. The godoc is explicit that `anonymous` /
`authenticated` built-ins do **not** exist yet because rela-server has no auth
layer. So "public read" = a role named `everyone`. No new identity concept
needed.

### Precedent: capability-gated history reads

`PermHistoryRead` ("history:read") and `PermHistoryReadRedacted`
("history:read-redacted") — `internal/acl/policy.go:44,56` — gate deleted-entity
history and historical-field reveal as *global named permissions* in a role's
`permissions:` list. A subject-aware sibling exists:
`Request.HoldsPermissionForEntity` (`internal/acl/resolver.go:260`), used by the
statemachine transition guard, where per-subject scope comes from a
role-conferring relation rather than being baked into the permission noun.

### The history gating rule — the precedent to REJECT

`authorizeHistoryRead` (`internal/dataentry/history_handler.go:103`) gates a
version read on the **live entity's** read verdict. Correct for history (history
is *about* the live entity) and exactly **wrong** for pointers: the entire point
is that `published` be readable by principals who may NOT read the live row.

### Search seam

`search.TypeScope` (`internal/search/types.go:97`) is `{AllowAll bool; Query
*store.GraphQuery}`, server-derived, resolved fail-closed by `ResolveTypeScope`
(exact → wildcard → deny; nil map denies everything). Any pointer perspective
must eventually adopt this shape — and inherits the relation-only `GraphQuery`
constraint.

### Constraints

- Versioning is **postgres-only** (fsstore gets versioning from git).
- Don't add methods to `store.Store` — use the injected-service pattern.
- Read-out routes through `visibility` wrappers; base readers stay ungated.
- No existence oracle: denied ≡ nonexistent.
- No user Lua on the read path (`internal/entitymanager/CLAUDE.md`).

## Options

### Storage of pointer state

**S1. Pointer table keyed (entity_id, name) → vseq.**

```sql
CREATE TABLE entity_pointers (
    entity_id     TEXT COLLATE "C" NOT NULL,
    name          TEXT NOT NULL,
    vseq          BIGINT NOT NULL,
    moved_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    moved_by_user TEXT NOT NULL DEFAULT '',
    moved_by_tool TEXT NOT NULL DEFAULT '',
    PRIMARY KEY (entity_id, name),
    FOREIGN KEY (entity_id, vseq) REFERENCES entity_versions(entity_id, vseq)
);
```

- **Pros.** Move = one UPSERT: atomic, cheap, auditable. The FK makes
"points at a real version" a DB invariant. Rollback = move it back. Inherits
history's independence from entity deletion (no FK to `entities`), which is
right — a published version should survive a live-row delete. Attribution
columns mirror the existing version-row convention.
- **Cons.** New table; postgres-only. Pointer-scoped list reads become a
join against candidates.
- **Effort.** Small — one migration, methods on the injected service.

**S2. Pointer as a property on the live entity** (`published_vseq: 1234`).

- **Pros.** No new table; portable to fs/mem.
- **Cons.** Conflates content with control state — and *circularly*, since
each version snapshot would capture the pointer value pointing at a version.
Writable through the ordinary update path (one field grant away from forgery).
No FK integrity. Pollutes the metamodel's user-facing property namespace.
- **Effort.** Small, but the coupling is disqualifying.

**S3. Pointer as a relation** (`entity --published--> version-entity`).

- **Pros.** The *only* option expressible in `store.GraphQuery` today
(`HasOutbound`), so ACL scope pushdown and search scope would work with zero
query-language change.
- **Cons.** Requires versions to be first-class graph entities, which they
emphatically are not — the survey confirms version rows form no graph and
deliberately carry no inter-version edges. Materializing every version as a node
is a large, distorting change.
- **Effort.** Large. Not recommended, but recorded because it is the only
option that fits the existing query language exactly.

### Declaration of pointers

**D1. `pointers:` block on `EntityDef` in `metamodel.yaml`**
(`internal/metamodel/types.go:218`).

```yaml
entities:
  page:
    pointers:
      draft:     {default: true}
      published: {}
```

- **Pros.** Pointers are a *shape* fact — which lifecycle rails a type has
— so the metamodel is the right home, consistent with `statemachine` transitions
already living there. Available to `analyze_*`, the SPA, and docs generation
without reading `acl.yaml`.
- **Cons.** Splits the feature across two files: shape in `metamodel.yaml`,
who-may-read in `acl.yaml`. (This is the existing convention, not a new wart.)
- **Effort.** Small.

**D2. Declare pointers in `acl.yaml`.**

- **Pros.** One file.
- **Cons.** Wrong layer — `acl.yaml` is optional and its absence means
allow-all, so a project without a policy would have no pointers at all. Makes a
structural concept contingent on an authorization file.
- **Effort.** Small, but architecturally wrong.

### ACL integration — the real question

**A1. Gate the pointer read on the live entity's read verdict** (the history
precedent).

- **Pros.** Zero new concepts.
- **Cons.** **Inverts the requirement** — `everyone` would need read on the
live row to see `published`, which simultaneously exposes the draft.
- **Effort.** Trivial, but it does not solve the problem.

**A2. Global named permission per pointer** (`pointer:read:published`),
following `PermHistoryRead`.

- **Pros.** Exact existing precedent; `permissions:` and `HoldsPermission`
already exist. Smallest change that works at all.
- **Cons.** Global and type-blind: "may read published pages" and "may read
published policies" collapse into one capability. `HoldsPermissionForEntity`
could add per-entity refinement via role-conferring relations, but not
per-*type* granularity.
- **Effort.** Small.

**A3. Make the pointer part of the read SUBJECT — a per-pointer read verdict.**
Qualify the type in a role's read grant, and resolve a separate
`ReadQueryResult` per (type, pointer):

```yaml
roles:
  everyone:
    read: ["page@published"]      # public sees only the published pointer
  author:
    read: ["page"]                # unqualified = the live/working row
```

Resolution: a read carrying pointer P for type T consults the grant for `T@P`;
an unqualified grant covers the live row only. `readQuery` gains a pointer
parameter; `AllowAll`/`DenyAll`/`Query` semantics carry over verbatim.

- **Pros.** Directly expresses the requirement. Preserves the *shape* of
the gate — the three-valued result and the pushdown query — so
`visibility.Reader`, `search.TypeScope`, and fail-closed `ResolveTypeScope` all
keep working with a widened key. Per-type AND per-pointer granularity.
`everyone` supplies "public" with no new identity concept. Composes with field
redaction unchanged.
- **Cons.** Widens the read-gate signature, so every call site threads a
pointer (defaulting to live). Grant-syntax change in `acl.yaml`. Needs
cross-file validation that `T@P` names a declared pointer (precedent exists:
`Policy.ValidateAgainstMetamodel`, and `internal/aclaudit` for the advisory tier
— note arch-lint forbids `acl → metamodel`, so the cross-check belongs in
`aclaudit`, not `acl`).
- **Effort.** Medium. The signature widening is mechanical but broad.

**A4. Add `When` to read grants.**

- **Pros.** Most general; pointer state becomes one predicate among many.
- **Cons.** A conditional read grant cannot be compiled into a
`GraphQuery`, so it degrades to per-row evaluation on list reads — exactly the
unbounded-hot-path pattern `internal/entitymanager/CLAUDE.md` forbids, and it
forfeits the SQL pushdown that makes list reads tractable.
- **Effort.** Medium-large, and it fights an existing architectural rule.

## Recommendation

**S1 + D1 + A3**: pointer state in a dedicated `entity_pointers` table reached
through the injected `VersionService`; pointer *declarations* on `EntityDef` in
`metamodel.yaml`; ACL via a per-pointer read verdict (`type@pointer`) in
`acl.yaml`.

Why:

- **A3 is the only option that expresses the requirement without inverting
it.** A1 fails outright; A2 is too coarse to distinguish types; A4 breaks
pushdown and contradicts the no-predicates-on-the-read-path rule. A3 keeps the
three-valued `ReadQueryResult` intact — which is precisely what lets
`visibility.Reader` and `search.TypeScope` keep working unchanged.
- **S1 keeps versions immutable and control state separate.** Because the
base is immutable, pointer resolution is a pure function — no locking, no
snapshot-coherence problem. The FK makes the invariant the database's job.
- **D1 puts shape in the shape file.** Pointers must exist even when
`acl.yaml` is absent (allow-all); D2 cannot provide that.

If effort must be cut, **A2 is the honest fallback**: it works, it follows an
existing precedent exactly, and its limitation (type-blind) is tolerable for a
single-content-type deployment. It is not a stepping stone to A3, though — the
grant syntax differs, so choosing A2 first means migrating policies later.

### Tradeoffs accepted

1. **Postgres-only initially.** Pointers ride on versioning. Non-postgres
builds get a nil `VersionService` and would serve only the live row — same
posture as history today. The git-backed fsstore path is a plausible later
analogue (a pointer *is* a ref) but is not free.
2. **Read-gate signature widening.** Every read call site gains a pointer
parameter defaulting to live. Mechanical but broad; the bulk of the effort.
3. **Cross-file validation.** `T@P` grants must be checked against declared
pointers; a typo (`page@publised`) would otherwise silently deny. Lives in
`aclaudit` (arch-lint forbids `acl → metamodel`).
4. **Search on a pointer is deferred.** `TypeScope` inherits the
relation-only `GraphQuery` constraint. Recommend excluding pointer-scoped search
from v1 and documenting it — "why doesn't my published page show up in search"
is a predictable complaint.
5. **`everyone` ≠ anonymous.** It includes authenticated principals. Correct
for "public", but once auth lands, `anonymous` vs `authenticated` should be
revisited (the `policy.go:31` comment already flags this).

### Blocking prerequisite discovered during the survey

**The MCP surface is entirely ungated on the read side.** Verified: zero
`visibility.` / `readGate` / `acl.` references anywhere in `internal/mcp/*.go`;
`internal/cli/mcp_wiring.go:66` wires `LuaWriteDeps()`, which carries
`visibility.Unrestricted`. The CLI and docs runtime are ungated too, but those
are a defensible operator trust boundary (whoever runs the binary already has
the project files). **MCP is a network-facing surface and is not.**

A draft/published split is a *confidentiality* feature. Shipping it while MCP
serves every entity ungated means the draft is protected in the SPA and readable
over MCP. This is not a pointer-design problem, but it is a gating prerequisite
for the feature's security claim, and should become a ticket in its own right
rather than being discovered at review.

Two further gaps, lower severity but worth recording:

- **Scheduled jobs get row gating only, no `visible:` field redaction**
(RR-7408F5, `appbuild.go:334-347`) — appbuild has no affordance resolver, so
`ScheduledLuaWriteDeps` passes a nil redactor.
- **`DenyTracer` refusal is invisible to Lua scripts**
(`denyreader.go:67-78`) — the three bound traversals return nil, which is
indistinguishable from a nonexistent id; only a `slog.Error` at the wiring site
signals it.

### Open question for the design phase

**Is field redaction on a pointer read computed against the pointed-to version
or the live row?** TKT-73C6B2 established that historical field redaction
**fails closed**: the live store no longer holds the as-of-version edges a
conditional `visible:` grant needs, so any `when:`-conditioned grant whose
subject-world inputs can't be affirmed hides the field. A pointer read hits the
identical wall.

Either accept fail-closed redaction on pointer reads (consistent with history,
but `published` content may over-redact — bad for a public surface, where
over-redaction is user-visible breakage rather than a safe default), or require
pointer-scoped `visible:` grants to be unconditional. Settle this before
implementation.

Related, and worth deciding at the same time: **relations have no field-level
redaction at all** (`FilterRelations` is row-gating only), and **bodies are
never redacted** (`policyreader.go:162-168` — the `visible:` universe is
metamodel-declared properties, so `Content` passes through verbatim). For a
genuinely public `published` pointer, the body is the main payload, so "no body
redaction" is a more consequential limit here than it is on today's
authenticated surfaces.
