# CLAUDE.md

## Interacting with tickets & docs

Use the `rela` CLI, not MCP. Pick the project with `--project=tickets` or
`--project=docs-project`.

```bash
rela --project=tickets list ticket --filter "entity.status == 'ready'"
rela --project=tickets show TKT-XXXX
rela --project=tickets create ticket -P title=... -P kind=chore -b '## Description ...'
rela --project=tickets update TKT-XXXX -P status=done
rela --project=tickets delete TKT-XXXX --force --cascade   # --cascade also drops its relations
rela --project=tickets link TKT-XXXX affects some-concept
rela --project=tickets unlink TKT-XXXX affects some-concept
```

- `-P/--property` takes `key=value` and is repeatable. It **splits on commas**,
  so escape any comma in a value as `\,` or the value is read as another
  property and rejected.
- `-b/--body` sets the markdown body; `-B/--body-file` reads it from a file or
  `-`. Both replace the whole body.
- `-o json` on any command when you need to parse the output.
- `--filter` takes a predicate expression; the older `--where key=value` is
  deprecated.

Before considering ticket work done, run the three gates and fix what they
report:

```bash
rela --project=tickets analyze cardinality   # missing required relations
rela --project=tickets analyze properties    # missing/invalid required properties
rela --project=tickets analyze validations   # custom rules (checklists, 5-whys, ...)
```

`analyze all` adds orphan/duplicate/gap reports; those are advisory and the
project has a standing backlog of them, so judge your entity by the three gates
above rather than by a clean `analyze all`.

## Rules for new code

- **Define interfaces at the call site, not next to the implementation.**
  Producer-side interfaces couple consumers to every method the producer
  exposes. Each consumer declares the minimum interface it needs (one to three
  methods). When a callback would create a constructor cycle, the consumer
  defines the narrow interface and the wiring site supplies it — see
  `docs/architecture/consumer-side-interfaces.md` and the godoc on
  `autocascade.Host`, `mcp.GraphReader`, `scheduler.WorkspaceProvider`.
- **Capability bundles, not service locators.** When a subsystem needs several
  collaborators, group them in a purpose-specific struct (see
  `internal/lua/deps.go` with `ReadDeps` / `WriteDeps`), split by read vs. write
  so read-only code can't accidentally mutate state. A scoped consumer-side
  `Services` interface is fine (see `internal/mcp/server.go`); a cross-subsystem
  grab-bag is not.
- **No repository or transaction abstractions.** Depend directly on
  `store.Store`, `tracer.Tracer`, `search.Searcher`, `entitymanager.Manager`.
  The old `repo` and `tx` layers are gone — do not reintroduce equivalents. The
  one sanctioned transaction seam is `store.Store.Tx` itself (DEC-8UIL0): a
  contract ON the store with per-backend meaning (fs/mem: write mutex, mutual
  exclusion only; postgres: native transaction + advisory lock, rollback, events
  at commit) — not a generic unit-of-work layer stacked above it. Don't wrap
  `Tx` in a new abstraction, and don't do slow external I/O inside a `Tx`
  callback.
- **Capture state once per operation.** Call `ws.Snapshot()` (or the equivalent
  `appState.Load()`) at the top of every handler, command, MCP tool, or
  observer; reuse the returned value for every read in that operation. Do not
  call `ws.Graph()` / `ws.Meta()` repeatedly — multiple loads against the
  underlying `atomic.Pointer` can observe different snapshots if a reload lands
  between them.
- **Don't leak storage or parsing types via return values.** A function that
  returns `*markdown.Document`, `*graph.Graph`, `interface{}`, or any type whose
  package the caller wouldn't otherwise need pulls the implementation into every
  consumer. Return value types or domain-package DTOs (`entity.Entity`,
  `entity.Relation`, `tracer.Result`). If you reach for `interface{}` plus a
  type assertion as a back-channel, define a typed dependency instead.
- **Split state-publish from write-serialize.** Use `atomic.Pointer[State]` for
  publishing a new state snapshot (no reader lock, no torn reads) and a separate
  `sync.Mutex` for serializing writers. Do not combine both responsibilities
  into a single `sync.RWMutex` — the lock-upgrade dance
  (`RUnlock → Lock → defer(Unlock → RLock)`) is the symptom, not the fix.
- **Constructors reject nil required fields.** A `New*` function with required
  collaborators returns `error` and validates them up front. Never substitute a
  no-op or sentinel implementation silently — that defers the failure to a
  downstream symptom that is much harder to diagnose.
- **Read-out paths go through visibility wrappers, base readers stay ungated.**
  Read-side ACL (entity row-gating + field-level `visible:` redaction) is
  enforced by `internal/visibility` decorators (`Reader`, the tracer decorator)
  injected at the wiring site — never by per-consumer redaction calls, and never
  inside `store`/`tracer`/ `search` themselves (DEC-ZBI39P; the
  `search.VisibleSearcher` pattern generalized). **Row-level**: a hidden entity
  is nonexistent — pruned subtrees, withheld paths, a denied GET
  indistinguishable from a real 404 (whether an entity _exists_ is a genuine
  secret). **Field-level** (`visible:`): redaction hides property **values
  only** — it makes no claim to conceal _which_ properties exist, since the
  metamodel (declared property names per type) is served over the API. A
  "field-existence oracle" is not a threat this guards against; code need not
  contort to hide field names, only their values. A system job that may read
  everything gets an `AllowAllReader` capability at wiring while keeping its
  genuine `system:*` principal for audit — allow-all is never inferred from
  identity. Write-prep reads (entitymanager diffing) keep raw store access: a
  redacted read-modify-write would clobber hidden fields.
- **Aggregates computed over the graph gate BEFORE they fold.** The gantt
  endpoint (`internal/dataentry/gantt_handler.go`, TKT-MW28U5) pins the pattern:
  row-gate the node set, redact each entity ONCE, then run the roll-up fold,
  then compute caps/`truncated` — all on the filtered tree. The `_views`
  pipeline's traverse-raw-then-redact-on-the-way-out order is safe for flat
  collections but NOT for an aggregate: folding raw values launders a hidden
  entity's dates into a visible parent's rolled span (a value disclosure, not
  the accepted one-bit membership channel), and a pre-filter count or truncation
  flag is an existence oracle (`TestGantt_ACLRollupExcludesHiddenChild`,
  `TestGantt_TruncatedIsPostFilter`). Any new derived-over-subtree value (sums,
  progress %, counts) follows the same order, and the result is per-principal —
  never cache it across principals. `visibility.Redact` is non-composable (raw
  store entities in, exactly once), so the redaction point stays single.
- **Partial writes go through `entitymanager.Manager.PatchEntity`, never
  read-modify-write.** Name the properties you are changing in an `entity.Patch`
  (`Properties` upserts, `MetaUnset` removes, `Content` is a `*string`
  tri-state) and the manager merges them against the raw stored entity
  internally. Properties you do not name are preserved — **forgetting one is a
  no-op, not an erasure.**

  The alternative — `GetEntity` → clone → merge → `UpdateEntity` — requires
  holding the _whole_ entity, so anything you failed to carry across is
  destroyed on save. That is unrecoverable when the read was redacted: a caller
  who cannot see a property cannot carry it, and silently deletes it. This used
  to be guarded by prose and a raw `lua.ReadDeps.WritePrepStore` handle;
  TKT-80EWGM removed both, so the mistake is now unavailable rather than merely
  discouraged. Pinned by `TestPatchEntity_PreservesUnnamedProperties` and
  `TestScriptReads_UpdatePreservesHiddenProperties`.

  `UpdateEntity` still exists for callers that legitimately own the whole entity
  (a form save that renders every field). If you are writing a _subset_, you
  want `PatchEntity`.
- **The configuration is not a secret; the data is.** `schema.yaml`,
  `data-entry.yaml`, `acl.yaml`, `schedules.yaml`, `scripts/`, `actions/`,
  `templates/` are operator-authored files that live in the repo — routinely a
  public one, as in any open-source app. Their _contents are already disclosed_.
  So list names, view/kanban/document/form/action names, entity and property
  names, `permission:` values, `script:` paths, even `command:` strings are
  **not** confidential, and code must not contort to conceal them: no filtering
  config endpoints per-principal, no indistinguishable-404-vs-403 on a _config
  key_, no narrowed wire types justified as leak prevention, no tests asserting
  a config name is unenumerable. The generalization of the `visible:` rule above
  (which already says property _names_ need no concealment because the metamodel
  is served over the API) — it holds for every config surface, not just the
  metamodel.

  What IS secret is **entity and relation content, and entity existence**. The
  read-path gating in `docs/acl-security.md` and the row-level rule above are
  about _rows_, not about the schema describing them. Keep the uniform 404 for
  an **entity id**; a 403 naming the missing permission is the right answer for
  a **config-declared capability**, and is more useful to the operator debugging
  it.

  This is settled, not open: `docs/acl-security.md` § "Sidebar menu structure is
  principal-independent" already records the decision. The menu carries no
  per-principal data: its structure is the same for every principal except for
  the opt-in `permission:` UX filter, and the rows behind an `entities:` entry
  (TKT-PEKL8L) come from the ACL-gated list API, not from the menu payload. The
  reason is that the metamodel is not a secret; it is served by
  `/api/v1/_schema`. Per-principal menu filtering is **deliberately not done**
  as a security measure. Don't reintroduce it as one.

  Two things this does NOT license. (1) _Secrets_ are not config:
  `.rela/secrets.yaml`, DSNs, and tokens stay off the wire — that is why
  `RELA_DATABASE_URL` is env-only. (2) A gate may still exist for
  non-confidentiality reasons — to keep an unusable entry out of a sidebar, or
  to stop an unauthorized caller triggering an expensive render — just don't
  justify it as concealment, because the next person will build on a secrecy
  property that was never real. Write down which of the two you mean.
- **Boundaries are enforced.** `just arch-lint` checks package import rules; run
  it before PR.

### Don't do this

- **Don't import `internal/graph` or `internal/model`** — both deleted. Use
  `internal/entity`, `internal/store`, `internal/tracer`.
- **Don't add a cross-subsystem service locator** (à la the removed
  `lua.Services`). Use `ReadDeps` / `WriteDeps` or a scoped consumer-side
  interface.
- **Don't call `ai.LoadProvider` directly from a new entry point.** Go through
  `script.NewWriterRuntime`, which calls `lua.LoadContextOptions`.
- **Don't wire AI into the validation path** — per-entity cost blowup with no
  quota. See `internal/ai/` docs for the rationale.
- **Don't reintroduce `internal/workspace`.** It was the legacy god-object
  aggregate; deleted in the workspace-decomposition arc. New code wires services
  individually via `appbuild.Discover` / `appbuild.New` or takes focused
  interfaces at the call site.
- **Don't run user-supplied Lua on the read path.** ACL gates evaluate against
  declarative policy + the graph; Lua participates only at write time. This
  targets unbounded/hot paths (per-row predicates on list reads), NOT a bounded
  single-subject evaluation the caller explicitly requested (e.g. performable
  transitions for one field on one entity). See
  `internal/entitymanager/CLAUDE.md`.
- **Don't add a zero-face read.** Store reads and writes take an
  `entity.Ref{ID, Face}`. A Ref with an id and no face names the zero-face
  row, which a faced type does not have (DEC-NPZICR). Use the entity's own
  `Ref()`, the face the resolver chose, or a parsed address.
  `internal/archguard/bareref_test.go` pins every bare `entity.Ref` literal in
  non-test code to an allowlist with a reason per file, and the list may only
  shrink. In tests, seed faced types only at their declared faces.
- **Don't pick a face for a bare-id write by rank.** A write that receives a
  bare id resolves it through `visibility.Resolver.WriteTarget` (or a reader's
  `WriteTarget`): exactly one readable face the world admits is the target,
  otherwise `*visibility.AmbiguousAddressError` names the faces. A read may
  take the face the world ranks first; a write may not, because the world's
  first face is a presentation choice, not the author's intent. Rename and a
  bare-id delete act on the whole family instead (`authorizeFamily`).

### Subsystem-specific rules (nested CLAUDE.md / godoc)

- **Writes, audit, ACL** → `internal/entitymanager/CLAUDE.md`. All writes go
  through `entitymanager.Manager`; do not write to `store.Store` directly from a
  write path.
- **Data-entry API + `_actions` affordances + write-validation policy** →
  `internal/dataentry/CLAUDE.md`.
- **Vue SPA build/test/architecture** → `frontend/CLAUDE.md`.
- **E2E tests** → `e2e/tests/AGENTS.md`.

Rules that apply to one subsystem live in `.claude/rules/*.md`. Each file names
the paths it covers in its `paths:` frontmatter, and Claude Code loads it when
you read or edit a matching file. If you change code in an area without first
reading a file the rule covers, read the rule file yourself. `tools/agentrules`
fails the build when a `paths:` glob matches no tracked file, so a move or
rename cannot silently unload a rule.

The table gives each file's one invariant that holds everywhere, including in
code outside its globs.

| Rule file             | Holds everywhere                                                                                   |
| --------------------- | -------------------------------------------------------------------------------------------------- |
| `acl-ceiling.md`      | No runtime deny: restrictions compile to allowlists when `acl.yaml` loads                          |
| `jobs.md`             | External side effects (mail, HTTP, AI) go on `jobs.Queue`, not inline on a write path              |
| `mail.md`             | Sanitize only the untrusted content, inline CSS last; a message's language is on `Message`         |
| `collection-reads.md` | No per-row lookups on a list path; read headers, batch per page, pin cost with a budget test       |
| `classification.md`   | `classification.yaml` describes data and never drives behavior                                     |
| `predicate.md`        | Conditions evaluate through `internal/predicate`; `condition:` and `when:` stay separate keys      |
| `transforms.md`       | Export follows an authorized view; external commands run only through `cmdexec`                    |
| `storage.md`          | Runtime state uses `state.KV`, not local files; test postgres dependencies via a schema-pinned DSN |
| `comments.md`         | Comments stay out of the graph: no entity type, audit, versioning or search                        |
| `versioning.md`       | History reads are gated like live reads; relation history on both endpoints                        |
| `datamigration.md`    | Migration steps stay idempotent; files are named by timestamp, never sequence                      |
| `configedit.md`       | Every config key is editable or locked; a Configure save replaces the server, never mutates it     |

## Architecture

rela is a schema-driven entity-graph platform. You define the shape of your
domain in a YAML metamodel (entity types, relation types, properties, validation
rules); rela gives you typed entities, typed relations, and tools to query /
validate / analyze / present the graph. Data is stored as markdown files with
YAML frontmatter.

Traceability (requirements → decisions → components) is one common use case, not
the identity. Other in-tree uses: ISO 27001 ISMS, project management, DevOps
runbooks, issue/ticket tracking (rela dogfoods itself — see `tickets/`),
documentation mirrors (`docs-project/`). Anything with typed entities and
relations fits.

```text
schema.yaml → Metamodel (entity types, relations, properties)
                     ↓
entities/*.md  → entity.Entity  ↘
                                 store.Store → tracer.Tracer  (pure reader)
relations/*.md → entity.Relation ↗          → search.Searcher (EntityObserver)
                                            → entitymanager.EntityManager
                                              (writes + automations + validation)
```

The store is the source of truth. `search` maintains a derived index as a
`store.EntityObserver`. `tracer` is a pure reader — no subscription, no derived
state. `entitymanager` is the "human intent" write path that runs automations
and validation on top of the store.

Write-path rules — validation policy (400/422/200-with-warnings), the audit log,
and ACL — live in the nested files `internal/dataentry/CLAUDE.md` and
`internal/entitymanager/CLAUDE.md`.

### Packages

Entry points: `cmd/rela`, `cmd/rela-server`, `cmd/rela-desktop`.

Domain and storage:

| Package                  | Purpose                                                                                          |
| ------------------------ | ------------------------------------------------------------------------------------------------ |
| `internal/entity`        | Domain types: `Entity`, `Relation` (no storage metadata)                                         |
| `internal/metamodel`     | Schema: entity types, relations, properties, validation                                          |
| `internal/store`         | Storage abstraction — CRUD + events; `fsstore`/`memstore`/`pgstore`                              |
| `internal/tracer`        | Pure-reader graph traversal (trace, path, orphans, cycles)                                       |
| `internal/relresolve`    | Answers `related(...)` in predicate programs: one gated store query per traversal per batch      |
| `internal/calfeed`       | Pure calendar-feed model + iCalendar/JSON serializers (event-granular; no store/vendor)          |
| `internal/mailrender`    | Pure message model → sanitized, CSS-inlined branded HTML + text/plain (leaf; no store/metamodel) |
| `internal/mail`          | Outbound email: `Sender` seam, SMTP + memory transports, `.rela/mail.yaml`, best-effort outbox   |
| `internal/classification` | Parse/lint/sync `classification.yaml`, the descriptive data-classification overlay (leaf; CLI only) |
| `internal/search`        | Full-text + structured search (bleve + linear)                                                   |
| `internal/visibility`    | Read-side ACL wrappers: row-gate + field-redact readers, tracer decorator (DEC-ZBI39P)           |
| `internal/entitymanager` | Write path: automations, validation, audit, policy                                               |
| `internal/audit`         | Append-only JSONL audit log of every successful write                                            |
| `internal/jobs`          | Background-job seam: ephemeral (fs/desktop) or durable (postgres)                                |
| `internal/principal`     | Identity attribution (`Principal{User, Tool}`) on ctx                                            |
| `internal/validator`     | Validation engine invoked by entitymanager                                                       |
| `internal/markdown`      | Parse/write entity and relation markdown                                                         |
| `internal/project`       | Project discovery, paths (`Context`)                                                             |
| `internal/appbuild`      | Wiring facade — constructs the focused services bundle                                           |

Subsystems (see each package's doc comment for details):

| Package                | Purpose                                                                                                    |
| ---------------------- | ---------------------------------------------------------------------------------------------------------- |
| `internal/cli`         | Cobra commands                                                                                             |
| `internal/mcp`         | MCP server over stdio — tools, resources, prompts, watcher                                                 |
| `internal/dataentry`   | Data entry web app (Go API + Vue 3 SPA in `frontend/`)                                                     |
| `internal/scheduler`   | Sequential Lua script scheduler (`rela scheduler`)                                                         |
| `internal/lua`         | Lua runtime + bindings (`ReadDeps`, `WriteDeps`)                                                           |
| `internal/script`      | Script execution helpers that wrap `lua` with project context                                              |
| `internal/automation`  | Automation engine invoked by `entitymanager`                                                               |
| `internal/autocascade` | Cascade orchestration (runs automation side-effects)                                                       |
| `internal/ai`          | OpenAI-compatible LLM provider (used from Lua)                                                             |
| `internal/migration`   | Schema migrations for project YAML files                                                                   |
| `internal/cmdexec`     | Safe external-command core (argv, no shell, `{in}`/`{out}`, timeout, cap) shared by attachment + transform |
| `internal/transform`   | View-export engine: markdown `Renderer` → external-tool format conversion (the `transforms:` registry)     |

Other packages under `internal/` are self-descriptive — ls the tree.

### Condition engine

`internal/predicate` is the typed expression engine behind every condition and
policy surface; `internal/filter` remains the query-filter DSL. The details are
in `.claude/rules/predicate.md`.

### View export & transforms

The `transforms:` registry turns view markdown into other formats through
confined external commands. The rules are in `.claude/rules/transforms.md`.

### Storage backends & build tags

The storage + search backend is chosen at compile time by Go build tags. The
composition root has one `New` recipe per scenario over shared
`prepare()`/`assemble()` helpers — see
`internal/appbuild/appbuild_{fs,memory,postgres,sqlite}.go` and the matching
`internal/cli/mcp_wiring_{fs,memory,postgres}.go`:

| Build tag         | Store         | Search                            | Binaries                                |
| ----------------- | ------------- | --------------------------------- | --------------------------------------- |
| _(none, default)_ | `fsstore`     | in-memory bleve                   | `rela`, `rela-server`                   |
| `memorybackend`   | `memstore`    | `LinearSearch`                    | (tests / experiments; no bleve)         |
| `postgres`        | `pgstore`     | PostgreSQL (`pg_trgm` + tsvector) | `rela-postgres`, `rela-server-postgres` |
| `sqlite`          | `sqlitestore` | SQLite FTS5 (`trigram`, in-DB)    | `rela-sqlite`, `rela-server-sqlite`     |

`sqlitestore` is the **single-process** backend (DEC-LFSYNY): one embedded
database file at `.rela/rela.db`, no server, and `Open` takes an exclusive
sidecar lock so a second process is refused rather than admitted. That refusal
is load-bearing — `unique:` is enforced by an untransacted scan in
entitymanager, so two writers would have no backstop and the violation would be
silent. It takes the strong `Tx` tier (rollback, post-commit-only events) and,
since TKT-4NU9ZD, content versioning too — so history comes from the database
rather than from git, which it cannot use because the markdown files are not the
source of truth. Since TKT-B51CYD its graph queries run as SQL, like pgstore's,
with `graphquerynaive` as the reference: `storetest.RunGraphDifferential`
compares the two on randomized queries for both database backends, and a name
the builder cannot render as a literal JSON path falls back to the naive
path. It also refuses to open on a filesystem where WAL cannot be
enabled (iCloud/Dropbox/SMB), because SQLite is unsafe there.

The rules for touching storage, comments, versioning and data migration are in
`.claude/rules/` (`storage.md`, `comments.md`, `versioning.md`,
`datamigration.md`).

## Tests

- Prefer table-driven tests with `t.Run(tc.name, ...)` subtests.
- Use `t.Helper()` on assertion helpers.
- `internal/store/storetest` provides the store conformance harness — any new
  `store.Store` implementation must pass it. Likewise any new
  `search.VisibleSearcher` implementation must pass
  `storetest.RunVisibleSearchTests` (the ACL-scoped search contract).
- Race detector is on in CI; don't add `//go:build !race` tags.

## Coverage

Go: `go-test-coverage` enforces **package floor thresholds** (no ratchet);
minimums live in `.testcoverage.yml`. Coverage within the floor is free to move
up or down — floors exist to catch "new untested package added" and "core
package silently lost its tests." The frontend has no coverage enforcement —
unit tests run plain (`npm run test:run`).

- Run locally: `just coverage-check`, `just coverage-html`.
- When a floor fails, add tests — don't lower the threshold without a reason.
- Use `// coverage-ignore: <reason>` sparingly, only for genuinely untestable
  code (main functions, external-tool dependencies, OS-specific paths). Reason
  is required.

## Lint

golangci-lint with project rules. Test files exempt from `dupl`, `funlen`, magic
numbers. Cobra `cmd`/`args` unused parameters allowed. Line length: 120.

**God-object load lines** (`just plimsoll`, CI job "God-object lint"). The
[plimsoll](https://github.com/sourcehaven-bv/plimsoll) linter caps three
independent surfaces — the metric that tracks a type accreting into a god-object
(`App`, `Runtime`, `FSStore` got there because nothing stopped them):

- **`max-methods` (40)** — total methods, exported + unexported. The backstop
  for internal sprawl: a receiver with dozens of private helpers is one struct
  whose fields they can all reach.
- **`max-exported-methods` (20)** — exported methods only. The sharper signal,
  since the public API is the coupling surface consumers bind to. Note these
  often diverge wildly from the total: `App` is 226 methods but only 13
  exported; the genuinely-wide _public_ APIs are the store implementations and
  schema value types (`FSStore`, `MemStore`, `Metamodel`).
- **`max-fields` (20)** — exported struct fields.

A new type over any line fails CI. Existing offenders are grandfathered with a
`//plimsoll:max-methods=N` / `max-exported-methods=N` / `max-fields=N` directive
at the declaration site, pinned to the current count so they can't grow; ratchet
those down as you decompose (TKT-N0IKN9). A store-implementation's exported
count is the mandated `store.Store` interface, so its directive is a documented
"required interface" exception rather than a ratchet target. Prefer splitting
the type over raising the number.

**Comment discipline** (`just comment-lint`, CI job "Comment lint").
[commentlint](https://github.com/sourcehaven-bv/commentlint) checks comments
against the scope they are attached to. `commented-code` and `doclink` are
**blocking gates** (both clean); the rest are advisory (`just comment-report`)
with a backlog being worked down:

- **`duplication`** — the same fact explained in two or more comments. The
  signal we act on: a fact stored three times gets corrected in one place and
  goes stale in two. Remedy is to hoist it to the type or package they cite.
- **`nil-contract`** — nil behaviour as ad-hoc prose. Go cannot express this in
  a type, so the convention is `Nil: rejected|accepted|never returned — <why>`.
  Fixing one removes it permanently (the rule skips tagged comments).
- **`doclink`** (gate) — a `[Bracketed.Reference]` that resolves to nothing. Go
  degrades these silently (pkg.go.dev renders the literal brackets) and no other
  linter catches them — `go vet`, `staticcheck` and `godoclint` all report zero
  on a broken link. Most are a bare `[Method]` where Go needs `[Recv.Method]`;
  the finding names the qualified form. Note Go cannot link an unexported member
  or a symbol from an unimported package at all — those references should simply
  lose their brackets.
- **`param-contract`** — a precondition asserted about a bare `string`/`int`
  parameter ("MUST already have passed containedPath"). Usually a missing type;
  this repo already does it where it matters most, e.g. `principal.Principal`
  keeps `roles` unexported so they can only enter through a verifying
  constructor.
- `too-long` and `scope-reach` are **off** — see `.commentlint.yml` for why.

False positives are expected (every rule is a heuristic over prose). Suppress
with `//commentlint:ignore <rule>  <reason>` on the declaration line, or via
`.commentlint.yml` when the same prose recurs across many sites.

**Read the finding before suppressing it.** A blocking check with an easy escape
hatch makes silencing the cheapest path to green, and a reviewer skimming a diff
cannot tell a considered suppression from a reflex one. Fixing the comment is
the outcome the gate exists for; suppression is for findings that are genuinely
_wrong_, and the reason must say why. "Suppressed to unblock CI" is not a
reason. The failure message says all this too, at the moment it matters.

## Security

`govulncheck` runs **daily** from `security.yml`, which auto-opens an
auto-merging `go get` PR when a fix exists and files a tracking issue when one
does not. It also gates releases (the `security` job in `release.yml`) — a
release must not ship a known vulnerability.

It deliberately does **not** run on the PR path. A vulnerability in an
already-merged dependency is not a defect in whatever PR happens to be in
flight, and gating there blocks unrelated work: the old `ci.yml` job scanned
only on a `go.mod`/`go.sum` diff, but the merge queue runs on `merge_group` (not
`pull_request`) and took the unconditional-scan branch — so an advisory would
pass on the PR and then dequeue it from the merge queue. Don't reintroduce a
blocking vulnerability check on PRs; raise the scan cadence instead.

Known-unfixable vulns are filtered via `scripts/govulncheck-filtered.sh` — keep
`IGNORED_OSVS` in sync with `scripts/govulncheck-fixable.sh`. Run locally:
`just govulncheck`.

## Commands

Read the `justfile` for the full set. The non-obvious ones: `just arch-lint`
(package boundary check), `just ci` (full pipeline), `just dev` (data-entry
server locally), `just coverage-check`. `go test -run TestName ./...` for a
single test. `just seqtrace-compare` traces ten demo requests on
`origin/develop` and on your working tree, and reports how the cross-package
call flows changed; use it to check that a change to a request path did what
you intended (`tools/seqtrace/README.md`).

## Project files

```text
schema.yaml                     # Entity/relation schema (was metamodel.yaml)
schedules.yaml                  # Optional: schedules for `rela scheduler`
entities/<type>/                # Markdown entity files by type
relations/                      # Markdown relation files (FROM--type--TO.md)
templates/entities/<type>.md    # Optional: entity templates for defaults
templates/relations/<type>.md   # Optional: relation templates for defaults
migrations/<stamp>-<slug>.yaml  # Optional: data migrations (committed)
migrations/applied.json         # Which migrations have run (COMMITTED, fs tier)
.rela/user-defaults.yaml        # Per-user defaults (gitignored)
.rela/scheduler-run-state.json  # Scheduler runs + last-run times, non-pg builds (gitignored)
```

## Working documents

Anything temporary — designs, tickets, QA notes, scratch — goes in `.ignored/`
(gitignored). Do not commit these.

<!-- @managed: claude-workflow start -->

## Rela for Planning & Issue Tracking

This project uses two rela instances via MCP for design and issue tracking:

- **rela-docs**: Documentation entities (concepts, features, guides, tutorials,
  scenarios)
- **rela-issues-and-design-tickets**: Issue tracking (tickets, features,
  decisions, concepts, risks, measures, tests)

### Workflow for Creating Tickets/Entities

When creating or updating entities in `rela-issues-and-design-tickets`:

1. **Create the entity** with required properties
2. **Run ALL analyze tools** to check for issues:
   - `analyze` with `check: cardinality` - check required relations
   - `analyze` with `check: orphans` - find unlinked entities
   - `analyze` with `check: properties` - validate property values
   - `analyze` with `check: validations` - run custom validation rules
3. **Fix any violations** (create missing relations, add required properties,
   etc.)
4. **Repeat analysis until ALL checks pass** - do not stop after fixing one
   issue

### Common Required Relations

| Entity Type          | Required Relations                                                                                                         |
| -------------------- | -------------------------------------------------------------------------------------------------------------------------- |
| ticket               | `affects` → concept (min 1), `implements` → feature (min 1)                                                                |
| feature              | `requires` → concept (min 1)                                                                                               |
| test-case/test-suite | `test-covers` → concept (min 1), `verifies` → feature/ticket (min 1)                                                       |
| doc-task             | `affects` → concept (min 1), `triggered-by` → ticket/feature/decision (min 1), `updates` → guide/tutorial/scenario (min 1) |
| research             | `researches` → concept (min 1)                                                                                             |

### Research Documents

For larger features, run `/research <topic>` before planning to survey
approaches and document tradeoffs. This creates a `research` entity (RES-xxxx)
with structured sections: Problem, Context, Options, Recommendation.

**Workflow:**

1. `/research` creates the entity in `in-progress` and links it to concepts
2. The agent surveys the codebase and external approaches
3. Options are documented with pros/cons/effort
4. A recommendation is made and presented for user review
5. The research is linked to the ticket/feature via `has-research`

**When to use:** Enhancements or features where the approach isn't obvious,
multiple viable options exist, or the change touches unfamiliar subsystems. The
planning checklist has a research item that can be skipped with N/A for smaller
work.

### Validation Rules

The metamodel includes validation rules that enforce:

- In-progress bugs should have `why1` and `why2` started
- Done bugs must have 5-whys analysis (`why1`-`why3` required) and `prevention`
- Ready tickets need `effort`, `priority`, and `description`
- Accepted decisions need `date`, `context`, and `consequences`

Always run `analyze` with `check: validations` to catch these issues.

### 5-Whys for Bug Analysis

Bug tickets use the 5-whys method for root cause analysis:

| Property | Purpose                          |
| -------- | -------------------------------- |
| `why1`   | What was the immediate cause?    |
| `why2`   | Why did that happen?             |
| `why3`   | Why did that happen?             |
| `why4`   | Why did that happen?             |
| `why5`   | What is the systemic root cause? |

Done bugs require at least 3 levels (why1-why3). The goal is to reach systemic
causes that can be addressed with process/tooling improvements documented in
`prevention`.

### Workflow Checklists

Tickets and bugs use workflow checklists to ensure thorough planning, execution,
and review. Each phase has a dedicated checklist entity with standard items from
templates.

**Ticket Workflow:**

```text
backlog → ready → planning → in-progress → review → done
                     │            │           │
                     ▼            ▼           ▼
              planning-      implementation-  review-checklist
              checklist      checklist        (+ docs-checklist
                 │                            for enhancements)
                 ▼
           /design-review
           (before impl)
```

**Bug Workflow:**

```text
backlog → ready → analyzing → in-progress → review → done
                     │            │           │
                     ▼            ▼           ▼
              bug-analysis-  implementation-  review-checklist
              checklist      checklist
```

**Checklist Types:**

| Checklist                  | Purpose                                                      | Required For                             |
| -------------------------- | ------------------------------------------------------------ | ---------------------------------------- |
| `planning-checklist`       | Understanding, research, approach, security, risk assessment | Tickets entering `in-progress`           |
| `bug-analysis-checklist`   | Reproduction, root cause, fix planning                       | Bugs entering `in-progress`              |
| `implementation-checklist` | Development, quality checks                                  | Tickets/bugs entering `review`           |
| `review-checklist`         | Automated checks, code review, verification                  | Tickets/bugs entering `done`             |
| `docs-checklist`           | Code docs, project docs, external docs                       | Enhancement/docs tickets entering `done` |

**Review Commands:**

| Command          | When to Use                               | Creates                                      |
| ---------------- | ----------------------------------------- | -------------------------------------------- |
| `/design-review` | After planning, before implementation     | `review-response` entities for design issues |
| `/code-review`   | During review phase, after implementation | `review-response` entities for code issues   |

**Agent Workflow for Tickets:**

Checklists are **automatically created** when tickets/bugs transition to
specific statuses. The `create_entity` automation with `if_exists: skip` ensures
no duplicates.

1. **Start Planning** (status: `planning`)
   - Planning checklist is auto-created and linked via `has-planning`
   - Work through checklist items: understanding, approach, security, test plan
   - Run `/design-review` to catch issues before implementation
   - Address all critical/significant design findings
   - Mark checklist `status=done` when complete

2. **Start Implementation** (status: `in-progress`)
   - Implementation checklist is auto-created and linked via
     `has-implementation`
   - Work through development and quality items

3. **Start Review** (status: `review`)
   - Review checklist is auto-created and linked via `has-review`
   - Run `/code-review` to perform thorough code review
   - Address all critical/significant code review findings
   - If enhancement or docs ticket, manually create `docs-checklist`
   - Complete all checks before marking done

4. **Complete** (status: `done`)
   - All linked checklists must have `status=done`
   - All checklist items must be checked or skipped with reason

5. **Create PR** (after `done`)
   - Run `/pr` to create PR and monitor CI until all checks pass
   - Fixes any CI failures (lint, test, coverage) automatically
   - The PR URL and CI status are NOT recorded in the review-checklist. They
     post-date it — `/pr` gates on the ticket already being `done` and
     validating clean, and a `done` checklist may have no unchecked items, so an
     item asking for the PR URL could only be satisfied by a PR that does not
     exist yet (TKT-UFV01M). GitHub records both; the branch and commit messages
     carry the ticket ID.

**Bug Workflow Automations:**

- `analyzing` → auto-creates `bug-analysis-checklist` via `has-bug-analysis`
- `in-progress` → auto-creates `implementation-checklist` via
  `has-implementation`
- `review` → auto-creates `review-checklist` via `has-review`

**Skipping Checklist Items:**

When an item doesn't apply, use strikethrough with a reason in parentheses:

```markdown
- [x] ~~API docs updated~~ (N/A: no API changes)
- [x] ~~Performance check~~ (N/A: documentation-only change)
```

Items without reasons will fail validation.

### Review Response Protocol

**Triggering Code Review:**

When a ticket/bug enters `review` status, run the `/code-review` command. This
invokes the cranky-code-reviewer agent to perform a thorough code review and
automatically creates `review-response` entities for each finding.

Alternatively, invoke the cranky-code-reviewer agent directly for ad-hoc
reviews.

**Creating Review Responses:**

For each finding from code review:

1. Create a `review-response` entity with:
   - `title`: Brief description of the finding
   - `finding`: Full description of the issue
   - `severity`: `critical` | `significant` | `minor` | `nit`
   - `status`: `open`
2. Link to ticket/bug via `has-review-response` relation

**Addressing Review Responses:**

| Severity    | Required Action                    |
| ----------- | ---------------------------------- |
| critical    | MUST be fixed before done          |
| significant | MUST be fixed before done          |
| minor       | Should fix, can defer with reason  |
| nit         | Optional, can wont-fix with reason |

When addressing a finding:

- Fix the issue in code
- Update status to `addressed`
- Document the `resolution` (how it was fixed)

When not addressing:

- Set status to `wont-fix` or `deferred`
- Document the `reason` (justification required)

**Validation Gates:**

Tickets/bugs cannot be marked `done` if they have:

- Open critical review responses
- Open significant review responses

Minor/nit findings may remain open with warnings.

### Automation Actions

Status transitions auto-create checklists (and similar side effects) via
automations declared in the project's `schema.yaml`. Action types (`set`,
`create_relation`, `create_entity` with `if_exists`) and interpolation patterns
(`{{new.property}}`, `{{entity.id}}`, `{{today}}`) are documented in
`docs/metamodel.md` and exemplified in the live `schema.yaml`. Read those rather
than relying on a copy here — a stale copy is worse than a pointer.

Common mistake: `{{entity.title}}` is wrong; use `{{new.title}}` for a property
of the triggering entity.

<!-- @managed: claude-workflow end -->
