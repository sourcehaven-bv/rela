---
id: RES-QKREXZZ
type: research
title: 'How should rela offer atomic creation of several entities and their relations (GitHub #1993)?'
summary: Build a bounded POST /api/v1/_batch on an entitymanager.CreateBatch engine (generalized CreateLinked, one store Tx, audit and automations after commit); refuse it on fs/mem with an advertised capability; rela.tx and in-Tx automations are follow-ups
status: done
---

## Problem

GitHub #1993 (TKT-OTM4ME6) asks for a public operation that creates several
entities and the relations between them as one unit. The use case is a typed
message: create a `message`, create its flavour entity (an `answer`), and link
`answer --supertype--> message` and `answer --answers--> ask`. Other readers
must see the whole aggregate after success and none of it after failure.

Today every public write is one entity or one relation. Even `POST
/api/v1/{plural}` with a `relations:` field is not atomic: it calls
`Manager.CreateEntity` and only then `writeCreateRelations`
(`internal/dataentry/write_handler.go:511` and `:548`). The PATCH endpoint has
the same gap, which `docs/data-entry/api-reference.md:505-527` documents. That
section is also stale: it still says the store has no transaction primitive,
although `store.Store.Tx` exists since DEC-8UIL0. Line 1075 lists cross-entity
atomic transactions as out of scope.

The question: which surface gives this guarantee, what in the write pipeline has
to change, and what is promised on backends that cannot roll back?

## Context

### The transaction seam

- `store.Transactor.Tx` (`internal/store/store.go:203-235`) gives `fn` a
transaction-bound view. All store calls inside `fn` must use the view.
Outer-store writes deadlock on fs/mem and bypass the transaction on pg. A nested
`Tx` on the view joins the open one.
- `store.ContextInTx` / `store.InTx` (`store.go:1787-1795`) mark the ctx.
Bindings that must not run inside a transaction already check it: Lua
`rela.oauth` (`internal/lua/oauth.go:82`) and attachment rendering
(`internal/lua/attachrender.go:104`).
- pgstore (`internal/store/pgstore/tx.go:60-98`): BEGIN plus a global
advisory lock. Rollback on error. In-process events are buffered in `txPending`
and delivered only after commit. Search is in-database (`pg_trgm` + tsvector),
so index rows commit or roll back with the data.
- sqlitestore (`internal/store/sqlitestore/tx.go:26-...`): BEGIN IMMEDIATE on
a pinned connection, events in `pendingEvents`, delivered after commit. FTS5 is
in-database, so the index is transactional too.
- fsstore (`internal/store/fsstore/tx.go:11-31`) and memstore
(`internal/store/memstore/tx.go:22`): one write mutex. No rollback, and events
and observer notifications (the bleve index, the SSE bridge, the realtime
trigger worker) fire inline per write. So a reader on fs can see a partial
aggregate while the `Tx` is still open.

### Prior art: multi-write manager operations in one Tx

These already run several manager-level writes against one view and can be
reused:

- **`entitymanager.CreateLinked`** (`internal/entitymanager/linked.go:92-115`)
is the closest precedent: one entity plus one edge, plus an optional caller
write, in one `Tx`. It is used by realtime promote
(`internal/appbuild/realtime_promote.go:75`), realtime outputs
(`realtime_outputs.go:375`) and task-item conversion
(`taskitemconversion.go:232`). Its shape is the template for a batch:
  - All checks that need no transaction run first, as the caller
(`prepareLinkedCreate`, `linked.go:135-155`): create ACL, field gate, relation
ACL, existing endpoint read gate, type tuple. The relation ACL can run before
the new entity has an id because a relation write is decided from the source
type, never the id (`acl.TestRelationWrite_DecisionIgnoresSourceID`).
  - Inside the `Tx` it rebinds the deps: `deps := t.m.deps; deps.Store = view`
and calls `createCore(txCtx, deps, ...)` (`linked.go:169-171`). Capacity and
ownership of the existing end are checked on the view before the entity write
(`checkLinkedRelation`, `linked.go:303-308`).
  - Audit, alias notification, automations and cascade run after commit
through `finishCreate` (`linked.go:109-113`). Their failure is `FollowUpErr`,
not an error, because the data is committed.
  - On a store that cannot roll back it reports `*KeptEntityError` and still
audits what stayed (`finishKept`, `linked.go:234-252`).
  - The new endpoint is passed as `created *entityFamily` so the relation
checks do not look it up in the store (`linked.go:276-291`).
- **Conversion** (`internal/entitymanager/conversion.go:246`), **copy**
(`copy.go:247`), **soft delete / restore** (`softdelete.go:95`, `:281`),
**ordered moves** (`manager_order.go:231`, `:293`) and the **cardinality
repair** (`cardinality.go:206`) each run their own multi-write critical section
on a view.
- **Data migration** (`internal/datamigration/run.go:296`) writes raw store
updates in batches inside `Tx`. It bypasses the manager (no ACL, audit or
automation), so it is not a model for a public API.

There is no generic "run these manager intents in one Tx" API. DEC-8UIL0 planned
one (`Manager.InTx(ctx, func(Mutator) error)` behind a Lua `rela.tx(fn)`
helper), but it was never built: TKT-GXHI8 shipped only the store contract, and
no `rela.tx` binding exists in `internal/lua`.

### What does not use the view today

For a batch to be atomic, every read and write in the critical section must go
through the view. These collaborators are bound to the outer store at wiring
time:

1. **ACL row gate.** `acl.Declarative` holds a `faceMatcher` set in
`NewDeclarative` (`internal/acl/declarative.go:49`, `:135`).
`Request.ReadableFacesMany` queries it (`internal/acl/faceverdict.go:136`). A
relation-scoped read verdict inside the `Tx` therefore reads committed state on
another connection. It cannot see an entity or edge written earlier in the same
batch. `CreateLinked` avoids this because it never gates the new endpoint. A
batch must do the same: an endpoint created by the batch is resolved from the
batch plan, not from the store.
2. **Manager endpoint lookups.** `lookupSource` and `requireVisibleTarget`
read `m.deps.Store` (`internal/entitymanager/manager.go:2271-2296`). Inside a
batch they must run on the view, or before the `Tx` for pre-existing endpoints
only.
3. **Automations and cascades.** `finishCreate`
(`manager.go:971-1031`) calls `Cascade.Process(ctx, &cascadeHost{deps: m.deps},
...)` with `Mutator: m.gated()` (`manager.go:1019-1024`). Both carry the outer
store. A cascade script that writes through that Mutator inside a batch `Tx`
would deadlock on fs/mem and escape the transaction on pg.
`cascadeHost.CreateEntity` also audits at once (`recordCascade`,
`cascadehost.go:80`, `:207`), so a cascade inside a rolled-back `Tx` would leave
audit rows for data that does not exist.
4. **Background jobs.** Cascades enqueue background automation scripts
(`internal/appbuild/automationjobs.go:217-259`). `jobs.WithDeferral` exists to
hold enqueues until commit, but no production code calls it yet
(`internal/jobs/deferred.go:32-38`). Without it, a durable postgres job could
start on another connection before the batch commits.
5. **Audit.** The audit log is a JSONL file or memory sink
(`internal/audit/filesystem.go`), not transactional. The manager emits records
through `recordEntityAudit` / `recordRelationAudit`
(`internal/entitymanager/audithook.go:21-60`). DEC-8UIL0 requires that a success
record never exist for a rolled-back write. Denied-write records are written
before any write and are not affected.
6. **Alias notification** (`notifyAliasesOfCreate`, called from
`finishCreate` at `manager.go:983`) is an outbound hook and must also run only
after commit.

### Side effects at commit versus rollback

| Effect | pg / sqlite today | fs / mem today | Work for a batch |
|---|---|---|---|
| Store events (SSE bridge, realtime triggers at `appbuild/realtime_worker.go:413`, changefeed) | delivered after commit, dropped on rollback | inline per write, no rollback | none on pg/sqlite |
| Search index | in-database, transactional | bleve observer, inline | none on pg/sqlite |
| Audit success records | emitted by the manager after its own `Tx` | same | collect per operation, emit after commit |
| Alias notification | after the write | same | emit after commit |
| Automations and cascades | run after commit, each in its own `Tx` | same | v1: run after commit (follow-up). Inside the `Tx` needs items 3 and 4 above |
| `jobs.Queue` enqueues | direct, deferral not wired | direct (ephemeral queue) | only needed if cascades move inside the `Tx`; then wire `jobs.WithDeferral` at the batch boundary |

### How atomicity is decided today

There is no store-level "can roll back" capability. Realtime promote decides it
per build tag at wiring: `atomic := p.join != nil`
(`appbuild/realtime_promote.go:56`), and `txChannelsFor` returns nil unless the
store is a database store (`realtime_promote_sqlite.go:20-33`). On fs it then
sets `RelateBestEffort` and the edge becomes a warning. That is fine for an
internal UX action, but a public "atomic" API needs an explicit, advertised
guarantee.

### Relation type rule (#1992)

Another change (for #1992) makes a relation whose endpoint types the schema
disallows always rejected. Today the PATCH and create paths write such an edge
with a warning through `TolerateTypeMismatch`
(`internal/dataentry/relations_modern.go:576`, `:936`;
`internal/entity/writeapi.go:144-155`). The batch endpoint must not offer that
flag. A type-disallowed edge fails the whole batch with a 422 that names the
operation. This also fits all-or-nothing semantics better than
write-with-warning. Other soft conditions from DEC-HWZHA, such as a required
property not set, stay warnings on the 200 response.

## Options

### Option 1: bounded REST batch-create endpoint (recommended)

`POST /api/v1/_batch` (name to be decided) with a body such as:

```json
{
  "entities": [
    {"ref": "m", "type": "message", "properties": {"title": "..."}, "content": "..."},
    {"ref": "a", "type": "answer", "id": "ANS-7", "properties": {}}
  ],
  "relations": [
    {"from": {"ref": "a"}, "type": "supertype", "to": {"ref": "m"}},
    {"from": {"ref": "a"}, "type": "answers", "to": {"id": "ASK-3"}}
  ]
}
```

An endpoint is either `{"ref": ...}`, an entity created earlier in the same
batch, or `{"id": ..., "face": ...}`, an existing entity. A caller-chosen `id`
is allowed for manual-id types, as on the single create. Generated ids are
minted inside the `Tx`. `HighestID` on the view sees the batch's own earlier
writes, and the global write lock means no other writer can take the same id.

**Engine.** A package function `entitymanager.CreateBatch(ctx, m, Batch)`. It
must be a function: Manager is at its method cap, the same reason given for
`CreateLinked`. It generalizes `CreateLinked` from 1+1 to N+M:

1. Before the `Tx`, as the caller: for every entity, `prepareCreate`
(`manager.go:920`): face, create ACL, field gate, file and external-ref
refusals, custom id. For every relation, the relation ACL and type tuple through
`prepareRelationFrom` (`manager.go:3094`). Batch-local endpoints are passed as
`created *entityFamily`. Existing endpoints go through `lookupSource` /
`requireVisibleTarget`, so a hidden endpoint is `target_not_found`, as on PATCH.
The first refusal stops the batch. Nothing has been written, and `WroteNothing`
is true.
2. In one `Tx` with `deps.Store = view`: run `createCore` per entity in
order, then for each relation run the capacity and ownership checks
(`checkRelationCapacity`, `checkOwningEdgeLeaving`) and `writeRelationCreate` on
the view. On a store that rolls back, any error discards everything. Collect
audit records and alias notifications in a slice.
3. After commit: emit the collected audit records and alias notifications.
Then run on-create automations and cascades per entity, in order, as follow-ups,
the same way `CreateEntity` does today. A follow-up failure is reported per
operation and does not undo the commit.

Step 3 is the main scope choice. Atomicity covers exactly the requested entities
and edges, not what automations derive from them. That matches every current
create path, where automations also run after the entity commit. Moving
automations and cascades inside the `Tx` is a separate, larger change: a
view-bound `cascadeHost` and Mutator, buffered `recordCascade` audit, and
`jobs.WithDeferral` wired for the first time. It also runs Lua under the
deployment-wide write lock, which DEC-8UIL0 allows only with an explicit policy
against external I/O.

**What must change for the transaction view.** Manager deps are rebound per `Tx`
as `CreateLinked` does. ACL row-gate reads happen before the `Tx`, and only for
pre-existing endpoints. Batch-local endpoints are never gated through the store.
No change to `acl.Declarative` is needed for v1. Writing a row the caller cannot
read is already possible on the single create, and the response is redacted per
entity by the existing serializer. Validation and template defaults run inside
`createCore` on the view. Unique checks go through `writeWithUniqueCheck`, whose
nested `Tx` joins. Store and search events need no work on pg/sqlite.

**Backend guarantee.** Refuse on fs and memory. The appbuild recipe knows its
backend (`appbuild_{fs,memory,postgres,sqlite}.go`). It passes an `AtomicWrites
bool` to dataentry (and to MCP, if exposed). This follows the `join != nil`
precedent and adds no optional store capability, which DEC-8UIL0 rejected for
`Tx`. Advertise the flag as `"capabilities": {"atomic_batch": true}` on `GET
/api/v1/_schema`. Configuration is not a secret, so it may be served to
everyone. On fs/mem the endpoint answers `501 atomic_batch_unsupported`. A
client can then pick its draft protocol (option 3) without guessing. A
`?guarantee=serialized` mode for fs could be added later if someone asks, but
silently offering "serialized, no rollback" under an atomic name is what the
issue warns against.

**Limits.** At most 100 operations (entities plus relations) per request. The
body is capped with `http.MaxBytesReader` at 1 MiB, giving `413 body_too_large`
as `/_piles` and `/_leases` already do. The single create handler has no body
limit today. `DisallowUnknownFields`, as on the single create
(`write_handler.go:460`). Duplicate or unknown `ref` values, and a ref used
before it is defined, are 400 `batch_invalid` with a JSON pointer.

**Error shape.** The existing problem+json body `v1.Error`
(`internal/apiwire/v1/responses.go:898`), with one `errors[]` entry
(`v1.FieldError`, `responses.go:930`) whose `source` is an RFC 6901 pointer to
the failing operation (for example `/entities/1/properties/title` or
`/relations/0/to`), plus a new `operation: {kind, index, ref}` extension. Status
codes follow the single-write ones: 403 for an ACL denial (naming the operation;
`denied-write` audit row), 422 `target_not_found` for a missing or hidden
existing endpoint, 422 `validation_failed` / `face_required`, and a
type-disallowed edge (code to match #1992), and 409 for an id or edge that
already exists. A 200 lists the created entities by `ref` with ids, each
redacted for the caller, warnings per operation, and follow-up errors per
operation.

**ACL per operation.** Each entity gets its own `OpCreate` check and field gate.
Each relation gets its own relation-create check. All are evaluated as the
caller. Bypass and elevation are not available on this surface. The `_actions`
contract is unchanged: the batch adds no verb, because it re-runs the existing
per-operation checks.

**Pros.** It reuses `CreateLinked`'s proven structure and the manager's existing
checks, so the batch and single writes cannot drift. The contract is small and
testable. It solves the issue on pg and sqlite. The same engine can later carry
`POST /{plural}` with `relations:` (making it atomic on pg/sqlite) and the Lua
helper.

**Cons.** Automation outputs are not part of the atomic unit in v1. fs and
desktop users get a 501. Creates only: no update or delete in a batch.

**Effort.** L overall (about 5-8 days): engine M, REST handler, wire types,
OpenAPI and docs M, capability flag S, backend tests S-M.

### Option 2: transactional Lua block (`rela.tx`) or an operator-authored action

Build what DEC-8UIL0 planned: `rela.tx(function() ... end)` runs the closure's
Mutator calls in one store `Tx` through a manager-level grouping API. Domain
HTTP stays operator-defined, through `POST /api/v1/_action/{id}` running a
script that calls `rela.tx`.

- **Pros.** Arbitrary create, update and delete. Domain-specific and
operator-controlled. Already a decided direction.
- **Cons.** It needs everything option 1 needs, plus a view-bound Mutator
inside a running Lua runtime. Every binding that does external I/O (`ai.*`,
HTTP, mail, `rela.oauth`, attachments) must refuse inside the block. Today only
oauth and attachment rendering check `store.InTx`. A long or looping script
holds the deployment-wide write lock (the pg advisory lock is global). The REST
client from the issue would still need an operator to write an action per
aggregate shape.
- **Effort.** M-L on top of option 1's engine, more without it.

### Option 3: client-side draft protocol (documentation only)

Document a recommended pattern: create entities with an `unsent` / `draft`
marker, link them, then flip the marker last. Readers filter on the marker.
Retries are idempotent through caller-chosen ids.

- **Pros.** No server work. Works on every backend.
- **Cons.** It does not give the transaction boundary the issue asks for.
Every reader has to filter, and orphans need garbage collection.
- **Effort.** XS (a docs section). Worth doing anyway as the fs/mem answer
next to the 501.

## Recommendation

Build option 1 on pg and sqlite, refuse it on fs and memory with an advertised
capability, and document option 3 as the fallback.

The engine is `entitymanager.CreateBatch`, a generalization of `CreateLinked`.
All caller checks run before the `Tx`. One `Tx` with `deps.Store = view` writes
every entity and edge. Audit, alias notification, automations and cascades run
after commit. This keeps all of DEC-8UIL0's rules: `Tx` is used directly, there
is no new abstraction, there is no slow I/O inside the callback, and there is no
success audit for rolled-back data. Automations moving inside the transaction,
and `rela.tx`, are follow-ups on the same engine.

Accepted tradeoffs: automation-derived writes are not atomic with the batch in
v1, and fs/desktop deployments get a 501 instead of a weaker guarantee.

### Proposed ticket breakdown

1. `entitymanager.CreateBatch`: plan, pre-`Tx` checks, one `Tx` on a
rebound deps view, post-commit audit, alias notification and follow-ups. Tests
on memstore for the checks. Rollback tests on sqlite and pg: no rows, no audit
success records and no store events after a failing last operation. (M)
2. Atomic-writes capability: an appbuild recipe flag passed to dataentry,
and `capabilities.atomic_batch` on `/api/v1/_schema`. (S)
3. `POST /api/v1/_batch`: wire types, limits, the error shape naming the
operation, OpenAPI. Rewrite the stale "Atomicity" and "Out of scope" sections of
`docs/data-entry/api-reference.md` (in `docs-project/`, since `docs/` is
generated). (M)
4. Docs for the draft/complete protocol on backends without rollback. (XS)
5. Follow-up: route `POST /{plural}` with `relations:` through
`CreateBatch`, so the single create with edges becomes atomic on pg/sqlite. (S)
6. Follow-up: automations and cascades inside the batch `Tx`: view-bound
`cascadeHost` and Mutator, buffered cascade audit, `jobs.WithDeferral` at the
boundary. (L)
7. Follow-up: Lua `rela.tx` on the same engine, with an `InTx` refusal on
every external-I/O binding. (M)

### Open questions for the owner

1. fs and memory: refuse with 501 (recommended) or offer the endpoint with a
documented "serialized, no rollback" guarantee?
2. Are automation and cascade outputs outside the atomic unit acceptable for
v1, or must they be inside it? If inside, ticket 6 moves into scope.
3. Surface: a generic `POST /api/v1/_batch` limited to creates in v1, or a
narrower name such as `/_create_aggregate`? Should MCP get the same tool?
4. Limits: 100 operations and 1 MiB, or other values? Should they be
configurable?
5. Should the batch be allowed to create a row the caller cannot read
afterwards? The single create allows it today, and the response is redacted.

### Owner answers (DEC-ODU65KR)

1. fs and memory answer 501 `atomic_batch_unsupported`; support is
advertised as `capabilities.atomic_batch` on `/api/v1/_schema`, decided once at
wiring.
2. Automations and cascades run after commit as follow-ups and are not part
of the atomic unit in v1. A follow-up failure is reported per operation.
3. `POST /api/v1/_batch`, creates only, no MCP tool in v1.
4. Fixed limits: 100 operations and a 1 MiB body.
5. A batch may create rows the caller cannot read afterwards; the response
redacts them.
