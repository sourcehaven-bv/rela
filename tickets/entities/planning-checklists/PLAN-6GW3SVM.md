---
id: PLAN-6GW3SVM
type: planning-checklist
title: 'Planning: Flows M5d: entity-change triggers in flows.yaml and delete internal/automation'
started: "2026-11-14"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:**

In scope:

- A new top-level `triggers:` map in `flows.yaml`. Each entry is one
  entity-change rule, keyed by its name, with `description`, `on` and
  `outputs`. Document order is evaluation order, as the `automations:` list
  order is today.
- Parsing and checking in `internal/flowcfg`; compilation and evaluation in
  a new package `internal/flowcfg/triggers`, the sibling of
  `internal/flowcfg/transitions`. It fires through `flow.Fire` as M5b does.
- The output vocabulary `set`, `create`, `relate`, plus the M5b exceptions
  as output kinds: `add_to_pile`, `notify`, `script` and `check`.
- A project migration in `rela migrate` that moves `automations:` from
  `schema.yaml` (and in-project includes) into `flows.yaml`.
- A hard cut: `automations:` in a schema file is a load error that names
  the migration.
- Deleting `internal/automation`; `internal/autocascade` keeps executing
  the fired outputs. `metamodel.AutomationDef` and its schema-load
  validations move to flowcfg.
- Migrating every in-repo project: `tickets/`, `examples/basecamp`,
  `internal/apptest/testdata/project`, `internal/configedit/testdata`,
  `internal/dataentry/testdata/golden/fixture`, and the inline schemas in
  Go tests.
- Docs: GUIDE-metamodel, GUIDE-lua-scripting and GUIDE-audit-log in
  docs-project, the flows guide, and the CLAUDE.md "Automation Actions"
  pointer.

Out of scope:

- Rewriting the tickets workflow as a stateful flow (TKT-XB4M29R, M5e).
- A separate `renamed:` trigger key. A rename keeps firing `updated`
  triggers (background actions only), as today.
- Applying the flow-output `visible:` template restriction or typed
  template values to triggers. Both would refuse or change existing
  automations. Trigger templates keep today's string interpolation.
- Porting the Configure automations editor to flows.yaml: a follow-up
  ticket. This PR removes the Configure automations view and routes and
  shows a note that triggers live in flows.yaml, so no dead editor ships.

**Acceptance Criteria:**

1. `internal/automation` has no callers and is removed; `just arch-lint`
   passes with the `automation` component deleted.
2. The M5b goldens (tickets, docs-project, basecamp, dataentry fixture,
   synthetic) move to `internal/flowcfg/triggers/testdata/golden`, replay
   against `flows.yaml` produced by the migration, and are byte-identical to
   the committed files.
3. `rela migrate` converts every in-repo project; a second run reports
   nothing to do and changes no byte.
4. A schema with `automations:` fails to load with the message below, in
   `rela validate`, `rela-server`, `rela db load` and the MCP server.
5. Audit records keep `triggered_by: automation:<name>`; background jobs
   queued before the migration still resolve their action by name and file.

## Research

- [x] For larger features: run `/research` to create a structured research doc
- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A. The approach follows M5c (statemachine folded into
`internal/flowcfg/transitions`); the options are listed under Approach.

**Existing Solutions:**

- `internal/projectsetup/migrate.go` `renameLegacySchema`: a project-level,
  cross-file step that runs inside `MigrateWithFS`. The new move follows it.
- `internal/migration`: per-file `yaml.Node` migrations that keep comments,
  and `Blocking` detections the metamodel loader refuses. The move needs
  two files, so it is a project step, not a registry entry.
- `internal/flowcfg/transitions` (M5c): the pattern for a compiled table
  under flowcfg, wired from appbuild (`CompileHeadlessFlows`).
- `internal/automation/compile.go` (M5b): the trigger table, guard and
  output rendering. It moves almost unchanged.
- No external library applies; this is an internal config move.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**

*Target shape in flows.yaml.* Example: the tickets rule after migration.

```yaml
triggers:
  ticket-planning-checklist:
    description: Create planning checklist when ticket enters planning
    on:
      entity: [ticket]
      property: status
      becomes: planning
    outputs:
      - create:
          type: planning-checklist
          properties:
            title: "Planning: {{new.title}}"
            status: in-progress
            started: "{{today}}"
          relate: { type: has-planning, from: subject }
          if_exists: skip
```

*Key mapping* (automation → trigger):

| automations: | triggers: |
| --- | --- |
| `- name: x` | map key `x` |
| `on.entity/property/becomes/from/created/updated/relation_created/relation_removed/deleted/faces/copy` | same keys under `on` |
| `on.when: [filter, ...]` | `on.where: [filter, ...]` |
| `on.condition: expr` | `on.when: expr` |
| `do: [...]` | `outputs: [...]`, same order |
| `set: p` + `value: v` | `set: { properties: { p: v } }` |
| `create_relation: {relation, to}` | `relate: { type, to }` |
| `create_entity: {type, template, properties, relation, if_exists}` | `create: {type, template, properties, relate: {type, from: subject}, if_exists}` |
| `add_to_pile`, `notify` | same key and fields |
| `lua: src` / `lua_file: f` + `capabilities`, `allow_acl_bypass`, `background`, `run_as`, `retry` | `script: { lua: src }` / `script: { file: f, ... }` with the same keys |
| `validate: [{check, severity, message}]` | trailing outputs `check: { where, severity, message }` |

`where:`/`when:` keep the two dialects under separate keys, as the predicate
rule requires. `when:` means a predicate in every other place in
flows.yaml (`start`, branches, `join`), so it means one here too.
`relate.from` is `subject` or `new`; in a state `create` the default stays
`new` (new → subject), and a trigger `create` must name it. Templates keep
`{{new.*}}`, `{{old.*}}`, `{{user.*}}`, `{{entity.id}}`, `{{entity.type}}`,
`{{today}}`, `{{now}}`. An unknown placeholder (for example the state
spelling `{{subject.title}}`) is a load error, not literal text. Inline
scripts keep `InterpolateSafeOnly`. A `do:` item with several keys becomes
several outputs in the engine's fixed order (set, relate, add_to_pile,
notify, create, script); a failed `create` skips only scripts of its own
item, so the migration refuses an item that combines `create_entity` with
`lua`/`lua_file` and asks the operator to split it. An unparseable `check`
stays a runtime warning, as today.

*Runtime.* `flowcfg.Parse` decodes and checks `triggers:` (names, keys,
types, relations, piles, copies, script paths, the deleted-only-background
and unique-background rules now in `metamodel/automationjob.go`,
`addtopile.go`, `automationcopy.go`, `externalref.go`, `reserved.go`, plus
the abstract-type expansion of `on.entity` and the single-type check on
`create.type` from `namespaces.go`). Triggers have their own entry point,
`flowcfg.DeclaresTriggers`/`ParseTriggers`, which needs no ACL policy and
does not switch on the flow engine, run store or `_flow-run` type; appbuild
keeps them in `base.triggers`, separate from `base.flows`. A flows.yaml
with only `triggers:` is valid.
`rela mcp` hot reload gains a flows.yaml watch; Configure's preview
(`cmd/rela-server/configure.go` Validate) parses flows.yaml against the
candidate schema; `schema.Analyze` takes the trigger rules so cleanup does
not treat a type used only by triggers as unused.
`triggers.New(meta, rules, opts)` is `automation.NewEngineFromMetamodel`
moved: it compiles `where`/`when` per type and returns a `*triggers.Table`
with `Process(ctx, Event) *Result`. `Result` keeps its field names, so the
goldens compare as they are. `appbuild.buildAutomation` takes the
compiled flows instead of `meta.Automations`; the background-job lookup
(`findBackgroundAction`, `hasDeletedTriggers`) reads the rules.
`entitymanager.Deps.Automations` keeps its role, with the narrow
consumer-side interface over `triggers.Event`/`Result`. The audit label
stays `automation:<name>` (a value, not a type name).

*Alternatives rejected.*
(a) Each automation as a headless flow under `flows:` with a one-state
run: it records runs and writes as `run_as`, which changes identity, audit
and cost on every write. (b) Keep `when:`/`condition:` verbatim: one key
with two dialects in one file. (c) Keep reading `automations:` from the
schema as a deprecated alias: it keeps two declaration sites and the
metamodel code the ticket deletes. (d) Re-serialize from typed structs:
loses the operator's comments; the AST move keeps them.

**Migration plan:**

1. *Where it runs.* A project step `moveAutomationsToFlows` in
   `projectsetup.MigrateWithFS`, after the per-file migrations. Per-file
   steps such as `acl-bypass-enum` still rewrite old automation syntax in
   schema.yaml, so they must run first. `CheckPending`/`rela migrate --check`
   report it like the schema rename, so CI fails on an unmigrated project.
2. *What it does.* Parse schema.yaml as a `yaml.Node`. Decode each
   `automations:` entry strictly first and refuse, naming the key, when it
   carries a key the schema loader ignores today (flowcfg decodes with
   known fields only). Refuse entries that use YAML anchors or aliases
   (flowcfg refuses them, and an alias would dangle). Rewrite keys per the
   table and append the entries under `triggers:` in flows.yaml, in order.
   Only the new block is encoded; the rest of flows.yaml is kept byte for
   byte, and flows.yaml is created when absent. Comments travel with their
   nodes. Then cut the `automations:` block out of the schema by its line
   range, so the rest of schema.yaml keeps its formatting. Script paths
   (`lua_file` → `file`) are copied byte for byte, because the job lookup
   compares them exactly.
3. *Includes.* Included files merge after the root, in include order. The
   per-file metamodel migrations (such as `acl-bypass-enum`) first run on
   in-project includes too, so old syntax there is rewritten before the
   move. An include inside the project directory without `namespace:` is
   then migrated like the root. An include outside the project, or one with
   `namespace:`, makes the migration refuse and name the file: the first
   may be shared with other projects, and the second carries qualified
   names (`lib.x`) that are not valid trigger keys. The guide documents the
   manual steps for both. No in-repo include has automations.
4. *Names.* Trigger keys follow today's automation name rule: non-empty,
   no `.` or `:`, unique. Names are kept byte for byte, because background
   jobs on a durable queue resolve their action by (name, lua_file) and
   audit queries match `automation:<name>`. An unnamed automation gets
   `automation-<n>`, with the next n not already taken; it cannot have
   background actions, which need a unique name. Every in-repo name passes.
5. *Write order and idempotency.* Write flows.yaml first (atomic replace),
   then schema.yaml. A second run finds no `automations:` and changes no
   byte. After a crash between the two writes, the rerun finds both: when
   every schema automation converts to a trigger whose decoded value equals
   the one already in flows.yaml, it only cuts them from the schema;
   otherwise it refuses and names the conflicting triggers. A `triggers:`
   entry already in flows.yaml under another name is kept, and the migrated
   entries follow it.
6. *No data migration.* No entity or relation changes, so there is no
   `migrations/<timestamp>-<slug>.yaml` file and the data shape hash is
   unchanged. If one were needed it would be timestamp-named.
7. *Hard cut.* It is a `Blocking` detection, `automations-to-flows`, in the
   migration registry, marked project-level so the per-file runner skips
   its Apply. `FSLoader.Load` refuses it for the root and checks includes
   too; `LoadWithoutMigrationCheck` ignores the key, so `rela migrate` still
   gets a metamodel for the context-aware per-file migrations. A new
   `FileTypeFlows` gives later trigger-syntax migrations a home.
   `automations:` in schema.yaml or an include is a load error:
   `schema.yaml: automations: are declared in flows.yaml under triggers:
   since this release (TKT-GWOBH2U); run 'rela migrate' to move them`. It
   applies to the root and to includes, and is not advisory: the runtime
   that ran them is gone, so loading would silently drop them.
8. *Database-held config (sqlite).* `rela migrate` is filesystem-only. The
   error message adds, when the config came from the database: run
   `rela db dump <dir>`, `rela migrate` in `<dir>`, then `rela db load
   <dir>`. flows.yaml is already in `configRootFiles`.
9. *In-repo projects.* Run the migration over each project listed in
   Scope and commit the result; Go-test inline schemas are rewritten in the
   same commit.

**Files to modify:**

- New: `internal/flowcfg/triggers/*` (from `internal/automation`),
  `internal/flowcfg/triggers.go` (raw decode + check),
  `internal/projectsetup/automations_to_flows.go` (+ tests).
- Delete: `internal/automation/`, `metamodel.AutomationDef` and the
  automation checks in `internal/metamodel` (moved to flowcfg).
- Change: `internal/autocascade/*`, `internal/entitymanager/{manager,core,
  cascadehost,copy*,conversion_plan}.go`, `internal/appbuild/{appbuild,
  automationjobs,schemacheck,notify_recipients,deletedstate,flows}.go`,
  `internal/appbuild/appbuildtest/fixture.go`, `internal/lua/deps.go`,
  `internal/audit/provenance.go`, `internal/schema/analyze.go`,
  `internal/testutil/fixtures.go`, `internal/configedit/allowlist.go`
  (drop `automations.*`), `internal/metamodel/loader*.go` (hard-cut error),
  `.go-arch-lint.yml`, the projects listed in Scope, docs-project guides.
- Also: `internal/appbuild/flowruntime.go` (`base.triggers`),
  `internal/cli/mcp_wiring.go` (flows.yaml watch),
  `cmd/rela-server/configure.go` (preview parses flows.yaml),
  `internal/cli/analyze.go`, `internal/mcp/tools_analysis.go` (Analyze
  input), `internal/migration/{migration,runner}.go` (project-level
  detection, `FileTypeFlows`), and the Configure frontend
  (`frontend/src/configure/{models,routes,references}.ts`,
  `views/configure/AutomationView.vue`, `router/index.ts`, sidebar).

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**

- flows.yaml `triggers:` (operator-authored config). Decoded with known
  fields only; every key is checked against the metamodel at load, and a
  bad rule is a load error, never a skip (dropping a `where` would widen a
  rule).
- schema.yaml and includes read by `rela migrate` (operator shell). The
  migration writes only inside the project directory and refuses
  includes outside it.

**Security-Sensitive Operations:**

- Cascade writes skip the acting user's ACL (system policy) apart from
  the managed-type denial. Unchanged; the trust root moves from one
  operator file to another of the same class.
- Scripts with `capabilities:` and `allow_acl_bypass`. The keys and their
  checks move unchanged; none is defaulted on by the migration.
- Background jobs run as `run_as` with the declared capabilities.
  Unchanged, and their (name, file) lookup is kept stable.

**Threat model (POLICY-015 §3, PROCEDURE-j5mq step 1).** Assets: entity
data written by cascades, the scripts' ambient capabilities, the audit
trail. Entry points: flows.yaml, schema.yaml, `rela migrate`, the Configure
editor. Threats and mitigations: (1) The migration widens a rule, for
example a `when:` filter becomes a predicate that matches more: `where:`
keeps filter syntax verbatim and the goldens pin the verdicts. (2) The
migration enables a capability or bypass: keys are copied as written, and
a test asserts a migrated script has exactly its old `capabilities:` and
`allow_acl_bypass`. (3) Automations silently stop running after an
upgrade: the hard-cut load error stops the project instead, and
`rela migrate --check` fails CI. (4) A queued background job runs another
action after the move: names are preserved and the (name, file) lookup is
unchanged, so it resolves to the same action or to none. (5) The
migration writes outside the project or into a shared include: it refuses
those includes. (6) Configure writes triggers it should not: the
`automations.*` keys leave the allowlist, the automations view is removed,
and flows.yaml is not a Configure file, so Configure can write nothing
there (default-locked). (8) Template injection into scripts: inline `lua`
keeps safe-only interpolation, and unknown placeholders are refused. (7) Audit
continuity: `triggered_by` stays `automation:<name>`. No new trust boundary
and no new network or file sink.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:**

1. Build and `just arch-lint` with the `automation` component removed.
2. Golden test in `internal/flowcfg/triggers`: run the migration on each
   golden schema into a temp dir, load the produced flows.yaml, replay the
   M5b event matrix, compare with the committed `.jsonl` files unchanged.
3. Migration tests on a memory FS: each in-repo project converts; a second
   run is a no-op (byte-identical files); the committed migrated projects
   equal a fresh migration of their pre-change version.
4. Loader tests: `automations:` in the root and in an include yield the
   hard-cut error from `metamodel` load, `rela validate` and
   `appbuild.CheckSchemaCompiles`.
5. The existing automation and autocascade tests are ported to
   `triggers.Rule` and pass unchanged in assertions;
   `automationjobs` tests cover a job queued under the old name.

**Edge Cases:** comments kept on moved entries; flows.yaml absent, empty,
with `flows:` only, or with `triggers:` already; crash between the two
writes; unnamed automation; automation in an in-project include; a
`validate:`-only automation; `deleted:` with a background script; an
automation with both `when:` and `condition:`; `set` actions repeated in
one `do:`; templates with `{{old.*}}`.

Also: a multi-key `do:` item (split in the fixed order, covered in a
golden); a trigger on an abstract type; a flows.yaml with only
`triggers:` (no flow engine, no `_flow-run`); MCP reload after a
flows.yaml edit; schema cleanup keeps a type used only by a trigger.

**Negative Tests:** duplicate name; name with `.` or `:`; include outside
the project; namespaced include; conflicting `triggers:` entry after a
crash; unknown key under an automation or a trigger; an automation using a
YAML alias; an item combining `create_entity` with `lua`; `relate.from`
with another value; a trigger `create` without `relate.from`; an unknown
template placeholder; `where:` with an unparseable clause; an abstract
`create.type`.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**

- Behavior drift in the move: mitigated by the byte-identical goldens and
  by moving code rather than rewriting it.
- Test churn (about 150 uses of `automation.*` types in entitymanager
  tests): mitigated by keeping field names in `triggers.Rule`, so the port
  is mostly mechanical.
- Operators upgrading without running the migration: the hard-cut error
  names the command; `--check` catches it in CI.
- Review size: the PR is large. Configure is split off to keep it
  reviewable.
- Effort: xl (the ticket says l; the Configure split keeps the rest at l+).

## Documentation Planning

For enhancements: identify what documentation needs updating.

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**

- [x] docs/metamodel.md: the automations section moves (GUIDE-metamodel)
- [x] docs/flows.md: new "Triggers" section (flows guide)
- [x] docs/lua-scripting.md and docs/audit-log.md: point at flows.yaml
- [x] docs/cli-reference.md: `rela migrate` mentions the move
- [x] CLAUDE.md: "Automation Actions" points at flows.yaml
- [x] ~~README.md~~ (N/A: no project-level change)

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** 13 findings, no critical, all addressed in
this plan. Significant: RR-AJ1WG1F (triggers-only flows.yaml),
RR-0S1YNMN (multi-key `do:` items), RR-Y3PBSS6 (abstract types),
RR-X29DNI9 (hard cut vs. the migration's metamodel load), RR-1FP07FP (MCP
reload, Configure preview, schema.Analyze), RR-TWCLNTN (Configure editor),
RR-EVL8YBT (name rule). Minor: RR-DVHQ81E, RR-0ONDDI5, RR-6XQYYD3,
RR-PTR0U5E, RR-J3IFCB1, RR-1ULY95T.
