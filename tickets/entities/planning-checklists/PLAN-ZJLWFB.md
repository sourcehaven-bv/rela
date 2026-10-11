---
id: PLAN-ZJLWFB
type: planning-checklist
title: 'Planning: Implement in-app configuration editing (Configure space)'
started: "2026-10-04"
completed: "2026-10-05"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

Operators edit the data model (`schema.yaml`) and the screens
(`data-entry.yaml`) from a built-in "Configure" space in the SPA, as in the
Storybook mockups (`Mockups/Configure data model`, `Mockups/Configure screens`).
Edits collect in a draft, are reviewed as a domain-worded change list, and are
saved in one step. When a change does not fit existing records, saving also
generates the data migration, writes it, and runs it. YAML never appears in the
UI.

User decisions (2026-10-04): one ticket for everything; no schema-as-graph
(FEAT-KXV0YJ is not a prerequisite); the server may persist and run migrations
(amends the "Gate.Persist is CLI-only" rule, recorded as a decision); editing is
off unless the server is started with an explicit flag, and then needs a new
`config:edit` permission.

**Scope:**

IN:
- Backend: `GET/POST /api/v1/_configure*` endpoints (read, preview, save,
impact counts, migration history), the comment-preserving YAML write, the
editable-path allowlist, validation before write, optimistic concurrency,
migration generation + run, and a live rebuild of the running app after a save
(schema and data-entry).
- `rela-server --config-editing` flag and `config:edit` built-in permission.
- SPA: the Configure space with every mockup screen: entity types, entity type
(properties, relations as sentences, settings), add/edit property drawer,
overview tabs (choice lists, relations, rules, automations), choice list,
relation, rule, automation, navigation, forms, form editor + field drawer with
transitions, list editor, board editor, dashboard editor, review drawer,
migration history. Previews as in the mockups.
- Draft kept in the browser (survives reload via localStorage, scoped per
project) until saved or discarded.
- Library: the mockup components already built (`RlSortableList`,
`RlChangeList`, `RlChangeItem`, icons, messages) and the `RlPageHeader` padding
fix; the mockup stories stay as documentation.
- Docs: data-entry guide section, data-migration doc, ACL doc, CLI/server flag
reference, CLAUDE.md rule amendments.

OUT:
- Schema-as-graph projection, MCP/CLI config editing tools.
- Editing security-sensitive constructs from the UI: `transforms`, external
`command:`s, `script:`/Lua references, actions, documents, custom apps,
`capabilities`, mail, schedules, `acl.yaml`. Screens show these read-only where
they appear (e.g. an automation's Lua action), never as editable.
- Schemas that use `includes:` (editing is refused with a clear message).
- Config stored only in the SQLite database (no disk file) and the postgres
build: the flag is refused at startup there (multi-node would diverge; DB config
has no write seam yet).
- rela-desktop.
- Renaming an entity type (exists as `rela rename entity`; a later ticket).

**Acceptance Criteria:**
1. Without `--config-editing`, no Configure entry is shown and every
`/_configure` endpoint returns 404. With it, a principal lacking an explicit
`config:edit` grant (`*` does not count) sees no entry and gets a
   403 response. The flag is refused at startup under `--read-only`, on the postgres
build, without a loaded `acl.yaml`, and without a verified identity source.
2. Opening Configure shows the current entity types, choice lists, relations,
rules, automations, navigation, forms, lists, boards and dashboard of the
project, in domain terms.
3. Edits on any screen go into one draft; the header shows the unsaved count;
the draft survives a page reload; "Discard all" clears it.
4. Review lists every change grouped by what it changes (added / changed /
removed), each row linking back to its screen.
5. Saving an additive draft (add property, option, field, column, nav entry,
reorder) writes the files with comments and untouched lines byte-identical,
rebuilds the running server, and the new property/field/column is usable
immediately without a restart. A new entity type with a plural is stored in its
own folder and still found after a restart.
6. Removing a choice option that records use requires choosing a target; saving
writes `migrations/<stamp>-<slug>.yaml` with a `map_values` step, runs it,
records it in `migrations/applied.json`, writes a `data-migration` audit record,
and the affected records now hold the target value.
7. Renaming a property generates and runs a `rename_property` migration; values
are preserved under the new name.
8. An invalid draft (dangling reference, bad expression, duplicate name) is
rejected at preview with readable errors shown on the screen it belongs to;
nothing is written.
9. If the files changed on disk after the draft was started, save is refused
(409) with "reload the configuration" guidance; nothing is written.
10. A draft that creates or changes a locked key is rejected (422) even if the
UI never produces it: `scan_cmd`, `transform[].cmd`, validation
`lua`/`lua_file`, automation `lua`/`lua_file`/`allow_acl_bypass`/
`capabilities`, `export_render`, `permission:`, `transforms`, `includes`.
Renaming or removing a name `acl.yaml` references is rejected (422).
11. A failure before the migration starts (validation, write, lock held)
restores every written file and the old app keeps serving. A failure after the
migration started rolls forward: files stay, the response says the migration is
incomplete, and retrying finishes it.
12. Migration history lists applied migrations with date, origin, author and
records changed; opening one shows its steps.
13. Every preview is labelled "Preview".
14. Every save, and every refused save, writes an audit record naming the
principal, file hashes and changed paths.

## Research

- [x] For larger features: run `/research` to create a structured research doc
- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A. The approach questions were settled with the user (see
decisions above) and the codebase survey is recorded below.

**Existing Solutions:**
- Comment-preserving YAML edits: `internal/migration/runner.go:104-168`
(yaml.Node AST, `SetIndent(2)`), `internal/migration/yaml_util.go`,
`classification/sync.go:54`. `metamodel/rename.go:84` and `schema/cleanup.go`
use plain `yaml.Marshal` (4-space reindent; not reused).
- Atomic file write: `storage.SafeFS.WriteFile` (`safefs.go:43-110`).
- Validation: `metamodel.Parse` + `worlds.Compile` + `computed.Compile` +
`dataentryconfig.ValidateConfig`; the strongest check is
`appbuild.NewSharedBase`. Errors are `[]string` without paths.
- Reload: data-entry.yaml hot-reloads (`watcher.go:338`); schema.yaml does not.
Reassembly pattern: `internal/cli/mcp_wiring.go:116-215` (`NewSharedBase →
ForReassembly().Assemble(store, searcher, …) → CloseAssembly`).
- Migrations: `datamigration.Generate` (text draft with CHANGEME),
`Resolve`, `NewRunner(Deps…)`, `audit.OpDataMigration`, `filemigstate`
(`migrations/applied.json`). Step structs are unexported.
- Counts: `store.GraphCount` with `PropPredicate{PropEqual}`.
- Wire format: every SPA endpoint is plain REST + JSON under `/api/v1` with
hand-written TS types in `frontend/src/api/*.ts`; new reads use Pinia Colada.
JSON-RPC or codegen would be a second style for one feature; not chosen.
- External: Strapi content-type builder and Directus data model editor both use
REST endpoints over a whole-schema document with server-side validation and
restart/reload; Prisma migrate drafts migrations from a schema diff and asks for
data decisions. Same shape as here.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**

*Wire contract (REST + JSON, the existing style).* The draft is the two
documents as JSON trees plus explicit intents the trees cannot express:

```
Draft {
  base_version: string            // hash of both files as read
  schema: object                  // schema.yaml as a JSON tree
  data_entry: object              // data-entry.yaml as a JSON tree
  renames: [{kind: property|option, entity_type?, choice_list?, from, to}]
  value_mappings: [{entity_type, property, from, to}]   // answers
  migration_title?: string
}
GET  /api/v1/_configure            → {version, schema, data_entry, editable}
POST /api/v1/_configure/preview    Draft → {errors[], changes[], migration?:
                                     {steps[], questions[]}, version}
POST /api/v1/_configure/save       Draft → {version, migration?} | 409 | 422
GET  /api/v1/_configure/usage?type=&property=&value=   → {count}
GET  /api/v1/_configure/migrations → [{name, title, applied_at, origin,
                                      author, records, steps[]}]
```

The client edits plain trees; TypeScript "screen models" map trees to what each
screen shows (a choice list = `types.<name>` in schema + `styles.<name>` in
data-entry). The server stays generic: it never needs per-screen code except for
intents and migrations.

*Write path (`internal/configedit`, new package).*
1. Read both files; `version = sha256(schema bytes ‖ data-entry bytes)`;
mismatch with `base_version` → 409.
2. Refuse when the schema has `includes:`.
3. Allowlist check: diff old tree vs new tree; every changed path must match
the editable allowlist (schema: `types`, `entities`, `relations`, `validations`,
`automations` minus script/lua/command-bearing actions; data-entry:
`app.name/description`, `styles`, `forms`, `lists`, `kanbans`, `dashboard`,
`spaces`). Inside those, `permission:`, `script:`, `command:` and action
references are never editable. Anything else → 422 naming the path.
4. Merge the new tree into the parsed `yaml.Node` AST: mappings matched by key
(key order taken from the new tree, so reorders work; removed keys dropped; new
keys appended in tree order); sequences of mappings matched by an identity key
(`name`, `id`, `property`, `group`, `label`) else by position; scalars replaced
keeping the original node style. Comments stay on their nodes, so they move with
reordered items. Encode with `SetIndent(2)`.
5. Validate the candidate bytes together (parse + compile + data-entry
validate against the candidate metamodel, then `NewSharedBase` on an in-memory
FS holding the candidate files plus the real other files).
6. Migration: `CompareShapes(stored, candidate)` + renames + value mappings →
steps. Exported typed constructors are added to `datamigration` (`NewFile(title,
from, to, steps)`, `RenameProperty`, `MapValues`, `RenameEnumValue` via
`map_values`) so the server builds the file as data, not by editing generated
text. A removed in-use option (drift tier today) still gets a `map_values` step,
because the UI requires a target.
7. Save sequence, under one save mutex and the migration lock: write the
migration file → write schema → write data-entry (SafeFS, atomic each) → run the
runner (persists applied.json, audits) → rebuild. On any failure after the first
write, restore the previous bytes of every file written and delete the migration
file; the old app keeps serving.

*Live rebuild (`cmd/rela-server`).* The handler served by the HTTP server
becomes a swappable root (`atomic.Pointer[http.Handler]`). A `Rebuilder`
(consumer-side interface in `configedit`: `Rebuild(ctx) error`, supplied by the
wiring site) re-runs the existing app build: `NewSharedBase` →
`ForReassembly().Assemble(old store, searcher, visible searcher)` → `NewApp` and
its setters → `StartWatching` → swap → stop old watchers, close old assembly.
The scheduler and the HTTP MCP handler are rebuilt the same way (the scheduler
gets a cancellable context). Open SSE streams on the old app close and the SPA
reconnects, then calls `schemaStore.reload()`. This avoids turning a dozen App
fields into atomics. The data-entry fsnotify reload is suppressed during a save
(the save rebuilds explicitly), so it cannot race the schema swap.

*Access.* `--config-editing` on rela-server (refused with `--read-only` and on
the postgres build); new built-in permission `config:edit`
(`internal/acl/policy.go`). Handlers: flag off → 404; no permission → 403. The
SPA learns it from a `configure: true` field on `/_sidebar`.

*SPA.* Routes `/configure/...` (exempt from the `/s/:space` prefixing);
`Sidebar.vue` shows the Configure groups on those routes and `SpaceSwitcher`
gets a Configure row when allowed. A `useConfigDraft` Pinia store holds the
trees, intents, unsaved count and change list; `queries/configure.ts` (Colada)
reads; `api/configure.ts` writes. Views under `src/views/configure/`, one per
mockup screen, built from rela-components; `RlConfigShell`/`RlConfigReview`/
`RlConfigPreview` move from fixtures into SPA components. Change wording is
computed client-side from tree diffs; the preview endpoint supplies migration
steps and validation errors.

**Alternatives considered:**
- Typed edit operations (one endpoint per action): more server code per
construct and a second source of truth for what an edit means. Rejected for the
generic tree merge + intents.
- Marshal Go structs back to YAML: loses comments, legacy syntax and defaults.
- JSON-RPC / OpenAPI codegen: a second API style for one feature.
- Restarting the process after a save: drops requests, needs a supervisor, and
does not work when run in a terminal.
- Per-field atomics in `App`: a large refactor of every handler; the swapped
root handler gets the same result.
- Server-side draft storage: unnecessary for a single editor; localStorage is
enough and the base-version check catches conflicts.

**Dependencies:** `gopkg.in/yaml.v3` (already used); no new Go or npm packages.

**Files to modify:**
- New: `internal/configedit/` (`tree.go`, `merge.go`, `allowlist.go`,
`validate.go`, `migration.go`, `save.go`, `handler.go`, tests, `testdata/`),
`frontend/src/api/configure.ts`, `frontend/src/queries/configure.ts`,
`frontend/src/stores/configDraft.ts`, `frontend/src/configure/models/*.ts` (tree
↔ screen models, change wording), `frontend/src/views/configure/*.vue`,
`frontend/src/components/configure/*`, `e2e/tests/configure.spec.ts`,
`e2e/pages/ConfigurePage.ts`.
- Changed: `internal/datamigration/` (exported step constructors, `NewFile`),
`internal/acl/policy.go` (`config:edit`), `internal/dataentry/api_v1.go` (+
`router_walk_test.go`, sidebar `configure` flag), `internal/dataentry/
watcher.go` (save suppression), `cmd/rela-server/main.go` (flag, swappable root,
rebuilder, scheduler/MCP rebuild), `frontend/src/router/index.ts`,
`frontend/src/components/common/Sidebar.vue`, `SpaceSwitcher.vue`,
`frontend/src/composables/useEvents.ts` (schema reload on rebuild), `.arch-lint`
rules for `configedit`, `.testcoverage.yml` floor, docs (`docs/data-entry.md`,
`docs/data-migration.md`, `docs/acl-security.md`, server flag reference),
`CLAUDE.md`.

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**
- Draft trees: size-capped body (2 MiB); every changed path checked against
the allowlist → 422; resulting files fully validated → 422 with messages.
- Renames/value mappings: names validated against the candidate metamodel
(type, property and value must exist) → 422.
- `migration_title`: length-capped, slugified through `MigrationName`.
- Usage query params: type/property must exist in the live metamodel → 400.

**Security-Sensitive Operations:**
- Writing config files: only `schema.yaml`, `data-entry.yaml` and
`migrations/<stamp>-<slug>.yaml` at fixed paths via SafeFS; no path from the
request.
- Code-execution surfaces (transforms, commands, scripts, Lua, actions, apps)
are outside the allowlist, so `config:edit` cannot be escalated to running
commands.
- Running a migration skips the ACL by design; it is reachable only through a
save that passed the flag + `config:edit` checks, and is audited with the real
principal.
- Usage counts read the raw store; they are only served to `config:edit`
holders and return counts, never entities.
- Errors name config paths and keys (not secret per CLAUDE.md), never entity
content.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:**
1. AC1: handler tests for flag off (404), no permission (403), read-only
refused at startup (main test); Sidebar unit test hides the entry.
2. AC2: Go golden test that GET over `tickets/` returns the trees; Vitest tests
for every screen model mapping on a fixture tree.
3. AC3: `configDraft` store tests (count, persist, discard).
4. AC4: Vitest change-wording tests per construct.
5. AC5: merge golden tests over copies of `tickets/schema.yaml` and
`tickets/data-entry.yaml` (add/remove/reorder at every level; diff limited to
the edited lines; comments kept); integration test with httptest + temp project:
save then create an entity using the new property.
6. AC6/AC7: integration tests on fs store: save → migration file present,
applied.json updated, entities rewritten, audit record present.
7. AC8: preview returns errors for dangling reference, bad expression,
duplicate name; files untouched.
8. AC9: modify file after GET → 409; files untouched.
9. AC10: tree change under `transforms`, a `command:`, a Lua automation action
→ 422.
10. AC11: inject runner failure → files byte-identical to before, old metamodel
still served.
11. AC12: migrations endpoint test + view test.
12. AC13: view tests assert the "Preview" label.
- E2E (`e2e/tests/configure.spec.ts`, own temp project per test): add an
option and save; remove an in-use option with a target and check the records;
reorder form fields and see the form change; nav entry added and visible in the
sidebar.

**Edge Cases:**
- Empty draft (save disabled; endpoint returns 400 "no changes").
- Reorder only (no value change) still writes and keeps comments with items.
- Option removed and re-added in one draft (no migration).
- Two renames chained (a→b, b→c) collapse to a→c.
- Unicode labels and colours; values with quotes/colons keep valid quoting.
- Concurrent saves: second waits on the mutex then gets 409 (version moved).
- File edited on disk while a draft is open: 409 on preview and save.
- Projects with no `data-entry.yaml` (created on first save of a screen edit).
- Large schema (tickets/: 2350 lines) preview under 300 ms.

**Negative Tests:** invalid JSON, oversize body, unknown top-level key,
non-allowlisted change, unknown type in a rename, mapping to a value that does
not exist, `includes:` schema, missing permission, flag off, stale version,
migration step failure, rebuild failure (old app keeps serving, error returned).

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**
- AST merge misattaches comments on delete/reorder (TKT-K58O37's open
question). Mitigation: comments stay on their own nodes; golden tests over the
real `tickets/` files at every level; refuse rather than guess when a sequence
has no identity key and its length changes mid-list.
- Live rebuild leaks goroutines or serves a half-built app. Mitigation: build
fully before the swap; close old only after; goroutine-leak test; on failure
keep old.
- Partial save (files written, migration failed). Mitigation: restore bytes;
integration test with an injected failure.
- Amending the CLI-only Persist rule. Mitigation: decision entity; only the
save path persists; audited.
- Scale: about 18 screens. Mitigation: shared screen-model layer and the
existing mockup components; one view per screen under 500 lines.
- Allowlist gaps let a sensitive key through. Mitigation: allowlist (not
denylist) with a test enumerating every top-level key of both config structs and
asserting each is classified.

Effort: xl.

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**
- [x] docs/data-entry.md (Configure space)
- [x] docs/data-migration.md (migrations generated by a save)
- [x] docs/acl-security.md (`config:edit`, flag)
- [x] docs/cli-reference.md and docs/server-security.md (`--config-editing`)
- [x] CLAUDE.md (Persist amendment, configedit rules: allowlist, rebuild)

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** security review and architecture review, both
2026-10-04. Critical/significant findings are review-response entities linked to
TKT-F5NGMG, all addressed by the "Design review amendments" section below (or
already by code in `internal/configedit`). Minor findings adopted: version check
under the save mutex and merge into exactly the hashed bytes, re-hash before
each rename, restore only when the file still holds our bytes; refuse the swap
if the ACL mode would drop from declarative to nop; Host/Origin/ JWT middleware
stays outside the swapped root; derived-index reconcile only after a successful
swap (and documented as restart-only for now); runner and validator errors
return the failing step and counts, details to the server log; tree depth and
node-count caps; previews single-flight; sqlite sweep and fs watcher gaps
documented.

## Design review amendments (supersede the approach above where they differ)

*Wire.* Mappings are `{"$m": [[key, value], ...]}` (ordered, token decoder). A
mapping inside a list carries its original index as a hidden `$i` pair; the
merge matches on it first, then identity keys, then position. New strings get an
explicit `!!str` tag. Anchors, aliases, merge keys and `includes:` are refused.
The write replays the edit as a three-way line merge over the original text, so
untouched lines are byte-identical.

*Allowlist.* Per construct, per key. Each editable construct lists its editable
keys; every other key is locked (create/change → 422; removing the whole item is
allowed). A guard test walks the yaml tags of every struct reachable from
`metamodel.Metamodel` and `dataentryconfig.Config` and fails on an unclassified
key. Renames/removals of names referenced by `acl.yaml` are refused.

*Access.* Flag refused without `acl.yaml` + verified identity, under
`--read-only`, and on postgres. `config:edit` is an explicit grant (no `*`), not
inherited by client baselines, documented as admin-equivalent. Migrations and
usage counts also need AllowAll read + property visibility + write grant on the
affected type. Usage `value` must be a declared option. New audit op
`config-edit` for saves and refusals.

*Migrations.* Orchestration moves from `internal/cli/migrate_data.go` into
`datamigration` and is shared with the CLI. Preview runs the gate first:
InSync/Adopted/Bootstrapped with nothing pending, else refused with guidance.
Generated files go through an exported spec type → marshal → `ParseFile` (step
types stay unexported). A removed option expands to `map_values` for every
entity property of that type; refused if a relation property uses it. The save
never persists drift. The save takes only its own mutex; the runner takes the
migration lock (`ErrLockHeld` → 409). Roll back only before the runner starts;
roll forward after (`POST /_configure/migrate` retries).

*Validation.* `configedit` declares `Validator` and `Rebuilder` interfaces;
`main` supplies them. Preview builds a `SharedBase` over an overlay FS holding
the candidate files plus the App's own config load, i.e. the same constructors
the rebuild uses, before anything is written.

*Rebuild.* `cmd/rela-server` extracts `buildApp(svc, proc)` that returns errors.
Process-level state is built once and injected: JWT verifier, security config,
upload locker and limiter, transform engine, webhook replay set, the
`configedit.Service` (save mutex). The fsstore gets an atomic schema
replacement. Order: stop the old App's config watch → write → migrate → assemble
+ buildApp → swap → broker sends `config-changed` and closes subscribers → drain
the old router's in-flight requests (after the save response) → stop and wait
for the scheduler → `CloseAssembly`. The remote MCP handler is rebuilt from the
new svc.
