---
id: PLAN-VKDTBP
type: planning-checklist
title: 'Planning: Declare data classification and policy applicability on the metamodel'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:**

In (slice 1, RES-TZH38L § What slice 1 delivers):

- Optional project file `classification.yaml`: `labels` (role, description,
reference, meta, `when:` combination rule for derived labels), `assign` (type →
field → labels | `none` | `needs-review`), `assign_relations`, overrides
`subject`, `subject_link`, `subject_hops`.
- Fields are declared properties plus `title` and `body`.
- Subject inference with provenance.
- `rela classification sync`, `lint`, `report` (text + JSON).
- `rela validate` runs the lint when the file exists.
- Advisory `rela acl audit` findings (per role, per client ceiling) with
minimisation hints.

Out: principal attribution as a built-in subject link, label suggestions in
`sync`, conformance profiles and `rela conformance check` (slice 2), UI badges,
ACL `@label` selectors, subject export/erase, `output` scope, any runtime
behaviour, any key in `schema.yaml`, core `searchable:`.

**Acceptance Criteria:**

1. **Absent file is inert.** A project without `classification.yaml`:
`rela validate` and `rela acl audit` output is unchanged (asserted with existing
fixtures/goldens committed before the feature code); `rela classification
lint/report` say the file is absent and exit 0; `sync` creates it.
2. **Lint.** Each defect is an error with path and line, and a non-zero exit:
missing field entry, `needs-review`, entry for an unknown type/field/ relation,
`title` entry on a type without a declared `title` property, `body` entry on a
type without content, undefined label in `assign` or a rule, reserved label name
(`none`, `needs-review`, role names), label name outside
`[a-z0-9][a-z0-9-]{0,63}`, unknown key at any level, invalid role, derived-label
cycle, `count.min < 1`, unknown scope, YAML anchors/aliases/ merge keys,
duplicate keys, non-string keys, any limit in § Limits exceeded. `rela validate`
includes these errors.
3. **Sync.** On the worked example: adds every missing field as
`needs-review` (declared property order where the schema has one, sorted
otherwise); keeps comments and key order (blank lines and custom indentation are
not kept; golden test shows the rewrite); resolves rename chains from
data-migration files and moves a label only when the old name is absent from the
current schema and the final name is present; reports stale entries and
conflicts without deleting; a second run changes nothing; the write is atomic
(temp file + rename).
4. **Report.** Worked example (person/employment/sick-leave): `employment`
carries `identified-person` at record scope; `person` and `employment` are
inferred subjects with provenance; `of` relations are subject links;
`sick-leave` carries `identified-health` at subject scope; overrides `subject:
{employment: false}` and `subject_link: {of: false}` change the result
accordingly; `person --manages--> person` does not merge two persons' profiles;
subject types whose ids are not opaque are flagged. JSON output is stable
(sorted) and carries structured `labels` and `fields`.
5. **ACL findings.** Evaluated per role for the role set `{role, everyone}`,
for each client baseline with no scopes, with each single scope, and with all
scopes:
   - `analyst` reading `employment{birth_date, postcode, gender}`: a finding
naming `identified-person` and the fields whose single removal breaks it; when
no single removal suffices, the finding says how many removals are needed.
   - A role reading `sick-leave.diagnosis`, `person.title` and the `of` edge
(both endpoints readable): a subject-scope finding naming `identified-health`.
   - `everyone` reading `person` and `analyst` reading `sick-leave`: the
finding for `analyst` includes the join.
   - A client baseline that removes `postcode` suppresses the record-scope
finding for that client.
   - Relation-property labels respect `RelationGrant.Visible`.
   - Findings are `low`; `rela acl audit --exit-code` with the default
threshold still exits 0.
6. **Isolation.** arch-lint allows only `cli` to import
`internal/classification`; the package has no internal dependencies.
7. **Offline.** All commands run from schema, `acl.yaml`, migration files and
`classification.yaml` only; they open no store, and work while a sqlite server
holds its lock.

## Research

- [x] For larger features: run `/research` to create a structured research doc
- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** RES-TZH38L

**Existing Solutions:**

- Libraries: none needed. Data-classification tooling (e.g. DLP scanners,
Apache Atlas classifications, OpenMetadata tags) classifies columns but ties
labels to enforcement or to a catalog server; the combination rule follows
ENISA's "ease of identification" and the quasi-identifier notion from
k-anonymity. yaml.v3 (already vendored) for strict node parsing and
comment-preserving rewrite.
- Codebase patterns:
  - `rela acl audit` (`internal/aclaudit/aclaudit.go:104,194`,
`internal/cli/acl.go:45-82`): finding shape, narrow metamodel interface, CLI
wiring, JSON output.
  - Strict YAML: `dec.KnownFields(true)` in `datamigration/steps.go:100-110`;
here a hand-walked `yaml.Node` for union values and line numbers.
  - yaml.Node editing helpers: `internal/migration/yaml_util.go:11-274`
(`GetMapValue`, `SetMapNode`, `RenameMapKey`, `InsertMapKeyAfter`).
  - Data-migration files: `datamigration.LoadDir` (`file.go:335`); rename
steps `renamePropertyStep` (`steps.go:130`), `renameEntityTypeStep` (:176),
`renameRelationTypeStep` (:631).
  - ACL read semantics: `roleGrantsRead` (`acl/readquery.go:213`), closed-world
`visible:` (`affordances/resolver.go:410-470`), relation readable iff both
endpoints readable (`acl/policy.go:707-711`), ceilings compiled in
`acl/ceilingcompile.go` (`ceilingFor` :137, `clamp` :259, `permitsRead` :396,
`FieldCeilingFor` :482).
  - Validate integration: `internal/cli/validate.go:63-85`.
- Concepts: `authorization`, `metamodel-types`, `audit-log`; FEAT-RCQ6SJ
(effective-access map), FEAT-OF2ZOL (principal → entity type,
`Policy.UserEntityType`, `policy.go:153`).

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**

1. **`internal/classification` (leaf package, no internal deps).** Depends
only on yaml.v3; the three yaml.Node helpers it needs are copied from
`internal/migration/yaml_util.go` (migration depends on storage). Never imports
metamodel or acl: the CLI hands it a `Shape` DTO (entity types with ordered
fields, declared-title flag, display properties, content flag, id type; relation
types with (from, to) pairs after alias/wildcard resolution, fields and content
flag).
   - `load.go`: `Parse([]byte) (*File, []Issue)` walks a `yaml.Node`;
rejects anchors, aliases, merge keys, duplicate keys and non-`!!str` keys; every
issue has a path and line.
   - `rule.go`: compiles `when:`; derived labels ordered topologically; a
cycle is an issue. `Eval(set)` and `Breakers(set)` (single-removal breakers; if
none, the minimum number of removals for the `count` case, else "several").
   - `lint.go`, `subjects.go`, `derive.go`, `sync.go`, `report.go` as before.
   - `Exposures(readable)`: given a principal view (readable fields per
type, readable relation (rel, from, to) triples and relation fields), returns
own labels and derived labels with breakers.
2. **Subject scope definition.**
   - A subject profile for subject type S is S's readable fields plus the
readable fields of every type T reached through readable subject links within
the hop limit.
   - One hop is one relation. Traversal continues only through non-subject
types and stops at any subject type, so a self-relation or a link to another
subject type never extends the profile (no cross-person merging).
   - Hop limit: default 1, per relation override `subject_hops`, max 3.
3. **Field semantics.**
   - `title`: an entry is allowed only when `title` is a declared property.
Otherwise the display title's labels are the union of the labels of its display
properties; a type whose display title falls back to the id is treated under the
id rule.
   - `id`: not assignable. The report flags subject types whose `id_type` is
not opaque (slug/derived ids), since the id then carries personal data.
   - `body`: assignable only when the type has content; readable iff the row
is readable.
   - Computed properties are ordinary fields (labelled on their own).
4. **ACL view through the runtime door (no second evaluator).**
   - The CLI builds `acl.NewDeclarative(policy, NullGraph{},
NullGraphQueryer{})` and, per analysed role and client view, a principal via
`principal.VerifiedFrom("classification-audit", ToolCLI, Claims{Roles: [role],
PrincipalType, Scopes})`, then `ForPrincipal` — the same path
`aclmap.MapPrincipalAs` (`mapprincipal.go:188-204`) uses. `everyone` is held
implicitly.
   - Type readability: one new method on `*acl.Request` in a new file,
`ReadsType(t string) bool`, using `roleFor`, `roleGrantsRead` and the ceiling's
`permitsRead`. Face-scoped grants count as the type (worst case, as
`roleGrantsRead` already does). The world axis is ignored: any world counts
(worst case).
   - Field readability: a static entry point in `internal/affordances`,
`(*PolicyResolver).StaticFieldVisibility(entityType)` and
`StaticRelationFieldVisibility(rel)`, reusing `dimension`, the cross-role opt-in
and the ceiling intersection, with every `when:` treated as passing and reported
as conditional. No closed-world logic is written in `acl`.
   - Relation readability per (rel, from, to): readable iff both endpoint
types are readable.
   - `aclaudit` consumes the result through a consumer-side interface; its
arch-lint deps stay `[acl]`. The CLI wires affordances and classification into
it.
5. **`internal/datamigration/renames.go`.** Exported `Rename{Kind, Owner,
From, To, Stamp}` and `(*File).Renames()`. Sync resolves chains in timestamp
order (type renames rewrite the owner of later property steps) before applying.
6. **`internal/aclaudit/exposure.go`.** `Exposure(views, labels)
[]Finding` with rules `C1-role-reads-label`, `C2-derived-record`,
`C3-derived-subject`, client variants tagged with the view; all `low`. `Audit`
unchanged; the CLI merges and sorts. `--no-classification` excludes them.
`Finding` JSON gains optional structured `labels`, `fields`.
7. **CLI.** `ClassificationCmd{Sync, Lint, Report}` in
`internal/cli/classification.go`. The commands use a light setup like `rela
validate` (`projectsetup` metamodel load + disk reads), not `readServices`, so
no store is opened. Files are read from disk under the project root, like `acl
audit`. If `classification.yaml` exists only in database-backed config, commands
refuse with a clear message. `sync` writes atomically. `rela validate` lints
when the file exists and the metamodel loaded. Wiring in `kong.go` (plimsoll
bump with reason, `requiresProject`).
8. **arch-lint.** Component `classification`, `mayDependOn: []`; only `cli`
lists it; comment stating the no-runtime rule. `cli` gains no new deps beyond
classification (affordances and acl are already allowed; verify).

**Limits** (each exceeded limit is a lint error; outputs that hit an enumeration
cap carry `truncated: true`):

| Item | Limit |
| --- | --- |
| File size | 1 MiB |
| Labels | 256 |
| Rule nesting depth | 8 |
| `subject_hops` | 3 |
| Breakers listed per finding | 20 |
| Types in one subject profile | 64 (then truncated) |
| Findings per role and view | 200 (then truncated) |

**Semantics fixed in this plan:**

- Findings are per role plus `everyone`. A principal holding several roles
may see less than one role alone under closed-world `visible:`; the finding text
says "role grants", not "user sees". Union analysis belongs to FEAT-RCQ6SJ
(`internal/aclmap`).
- Enabling classification adds `low` findings and, after `sync`,
`needs-review` lint errors until reviewed. Documented; `--fail-on low` users can
pass `--no-classification`.

**Files to modify:**

Create:
- `internal/classification/{doc,model,shape,load,rule,lint,subjects,derive,sync,report,exposure,yamlnode}.go` + tests
- `internal/datamigration/renames.go` + test
- `internal/acl/readstype.go` + test: `(*Request).ReadsType`
- `internal/affordances/static.go` + test: static field visibility
- `internal/aclaudit/exposure.go` + test
- `internal/cli/classification.go`, `internal/cli/classification_adapters.go` + tests
- `docs/classification.md`

Modify:
- `internal/cli/kong.go`: command field, plimsoll bump, `requiresProject`
- `internal/cli/acl.go`: merge exposure findings, `--no-classification`
- `internal/aclaudit/aclaudit.go`: optional structured JSON fields, exported sort
- `internal/cli/validate.go`: classification lint step
- `.go-arch-lint.yml`: component and deps
- `.testcoverage.yml`: floor for `internal/classification`
- `docs/acl-overview.md`, `docs/cli-reference.md`, `CLAUDE.md`

Delivery in three PRs: (a) package, parse, lint, sync, validate; (b) subjects,
derive, report; (c) ACL view, affordances static entry, audit findings.

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**

- `classification.yaml` (operator config): strict allowlist of keys at every
level; roles from a closed enum; label names validated as `[a-z0-9][a-z0-9-]*`;
reserved names rejected; rule depth and `count.min` bounded; unknown input is an
error, never ignored.
- Data-migration files: parsed by the existing strict `datamigration`
loader.
- Schema and `acl.yaml`: loaded by existing validated loaders.

**Security-Sensitive Operations:**

- ACL analysis must reflect client ceilings exactly. It goes through
`ForPrincipal` and `Request.roleFor` (the clamp point) and the existing
affordances field logic, so it cannot drift from runtime. Tests cover deny-read
wildcards, allow-list collapse, `visible`/`redact`, scope reopen, `everyone`,
face grants and relation-field `visible`. A bug makes the audit understate
exposure, which is advisory but misleading.
- The synthetic principal is built with `VerifiedFrom` inside the CLI only,
never from request input, and is tagged `classification-audit`.
- File write (`sync`): fixed file name under the project root, via the
project's storage FS; no path from input.
- No store is opened: every command works on schema and config files only,
so no entity values can appear in output.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:**

1. AC1: CLI tests on a temp project without the file: `validate` and
`acl audit` output compared with a golden from the current behaviour;
`lint`/`report` exit 0 with a notice.
2. AC2: table-driven `Parse`/`Lint` tests, one row per issue code, asserting
code, path and line; CLI test for non-zero exit; validate integration test.
3. AC3: sync golden tests: fresh file, partial file with comments, rename
via a migration file (property, entity type, relation type), conflicting rename,
stale key, idempotence (second run is a no-op).
4. AC4: report on the worked example fixture, text and JSON goldens;
override cases; hop limit 2 on a chain.
5. AC5: `ReadsType` and affordances static-visibility unit tests, plus an
agreement test comparing static visibility with `FieldVerdicts` on generated
policies (conditions true); `aclaudit.Exposure` tests with a fake exposer and
`acl.LoadPolicyBytes`; CLI test on the worked example with a client baseline,
`everyone` join, relation-field `visible`, `--exit-code` and
`--no-classification`.
6. AC6: `just arch-lint`.
7. AC7: CLI test on a sqlite project with the store locked by another
handle; test that no store constructor is called.

**Edge Cases:**

- Empty file; file with only `labels`; `assign: {}`.
- Type without a `title` property; display template title.
- Relation types with alias or wildcard endpoints; self-relations
(`person --manages--> person`).
- Derived label referencing another derived label; cycle; `count.min`
greater than the number of matching labels.
- Field carrying several labels, including the same label twice.
- A subject hub reachable from many types (cap on enumerated breakers).
- Sync on a project whose config is not file-backed.
- Rename chain `A→B`, `B→C`, then a new property `A` (must not be moved).
- Type rename followed by a property rename on the new type name.
- Rules where no single removal breaks the match (`min: 3` over 4).
- Profile limit reached on a hub type (`truncated: true`).

**Negative Tests:**

- Unknown key at each level; invalid role; reserved label names; label name
with uppercase or spaces; `when` with an unknown selector; scope other than
`record`/`subject`; waivers or profiles keys (slice 2) rejected as unknown.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**

- Per-role results overstate exposure for multi-role principals under
closed-world `visible:`. Mitigation: wording ("role grants"); union analysis
belongs to FEAT-RCQ6SJ.
- Drift between audit and runtime visibility. Mitigation: request built via
`ForPrincipal`; field logic reused from affordances; agreement test.
- Subject-scope combinatorics on hub types. Mitigation: 1-hop default,
bounded breaker enumeration, report truncation flagged.
- Relation-property renames have no migration step, so those entries become
stale after a rename. Mitigation: reported by sync/lint; documented.
- Plimsoll and commentlint limits on new structs. Mitigation: small types,
doc comments on every exported symbol.

- yaml.v3 rewrite loses blank lines and custom indentation. Mitigation:
documented; golden test.

**Effort:** xl, delivered in three PRs (see Files to modify).

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**

- [x] `docs/classification.md` (new): file format, field states, derived
labels, subject inference, commands, limits ("advice, not compliance")
- [x] `docs/cli-reference.md`: `rela classification sync|lint|report`
- [x] `docs/acl-overview.md`: C-rules in `rela acl audit`
- [x] CLAUDE.md: one rule: classification is descriptive, no runtime package
imports it; note that relations DO have field-level `visible:` redaction
(`RelationGrant.Visible`), correcting the current text

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** two reviewers (architecture; gaps and edge cases). 0
critical, 11 significant, 8 minor, 2 nit; all addressed in this plan.

- Significant: RR-YD58D0 (no second ACL evaluator), RR-3M3SPG (reuse
affordances visibility), RR-FKOCS3 (`everyone` role), RR-MDZ1XB (relation
triples and relation-field `visible:`), RR-87069L (subject scope definition),
RR-YNNN5G (rename reuse), RR-4TLY9V (world/face grants), RR-5O9A3G (scope sets),
RR-1R9V4J (limits), RR-YDX4G8 (no store, disk reads), RR-BP6P2K (YAML hazards).
- Minor/nit: RR-5FCJDV, RR-LNYU8L, RR-4381YG, RR-UUVJJ8, RR-0H4E6L,
RR-8N1KS1, RR-ZWW6IT, RR-1G8L1K, RR-BY5QJD, RR-27YTPY.
