---
id: PLAN-2430B4N
type: planning-checklist
title: 'Planning: Declarative peer pull: peers.yaml apply side'
started: "2026-11-08"
completed: "2026-11-08"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Problem:** steps 1 to 7 give a read-only diff between two instances and a peer
identity. Nothing applies the diff yet. Step 8 adds the declarative one-way
pull: `peers.yaml` names each source, and a run writes the rows the source
changed into this instance through `entitymanager`.

**Scope (in):**

- `peers.yaml` in the project root, parsed and checked against the local
schema (`rela validate`, the scheduler loader).
- Package `internal/peersync`: config, ledger, pull run, report.
- A run: diff the source against the ledger, fetch changed rows through the
peer read routes, map them, merge owned fields, write as `system:peer:<name>`,
record each row's external ref, handle gone rows.
- `on_gone: detach | delete` with a gone guard (threshold, empty-type guard,
failed auth is a failed run). `detach` is the default.
- Scheduler task `peer:<name>` when `every:` is set (job on `jobs.Queue`, run
state like subscriptions).
- `rela peer pull NAME [--dry-run] [--allow-gone N] [-o json]`.
- `httpremote.Remote.Get` for the peer read routes.
- Docs: peers.yaml guide section, CLI reference, threat model in the ACL
security guide. Fix doc issues #2157 and #2158.

**Scope (out, noted for step 9/10):**

- Two-way sync and pushing local edits (step 9).
- Status page, conflict list UI, `_events` early trigger (step 10).
- Faced types, attachments, comments, history.
- Three-way merge of relation meta (relation meta owned by the remote is
written as the remote says).
- The `sync/<peer>` version tag as base: see Approach, the base lives in the
ledger so every backend works.

**Acceptance Criteria:**

1. End-to-end: a source instance with a catalogue of entities and relations
and a peer key; a target pulls it. Entities and relations appear locally with
fresh local ids and an external ref per entity. A second pull with no change
writes nothing. Test: `TestPeerPull_EndToEnd` in dataentry (source router over
httptest with a peer key, target memstore manager).
2. Local additions survive an upstream update: a local property not in
`fields:` and a local relation stay after the source changes an owned field.
Same test, second phase.
3. A local edit of an owned field that the source also changed is a
conflict: the row is not written and is reported; other rows apply.
4. `peers.yaml` errors are reported at load with the peer and key: unknown
type, field not declared, ref property not `external_ref`, system shared by two
peers, faced type, manual ids, bad URL, bad credential name, unknown key. Test:
table test in peersync config tests and `rela validate`.
5. A run whose gone set exceeds `max_gone`, or that empties a type that had
linked rows, applies nothing and reports why; `--allow-gone N` lets an operator
confirm. Test: peersync run tests with a fake remote.
6. A 401 or 403 from the source fails the run; nothing is applied and the
ledger is untouched. Test: fake remote answering 401.
7. Remote input is validated: id and type of a fetched row match the request,
bodies are capped, values are coerced to the local field types, and the remote
never picks a local id. Tests: hostile fake remote.

## Research

- [x] For larger features: run `/research` to create a structured research doc
- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** RES-G8CYB1R (feature research), DEC-81161C3 (peer identity).

**Existing Solutions:**

- `internal/subscription` + `internal/scheduler/subscriptions.go`: the model
for a declarative pull (yaml in the project root, one scheduler task per entry,
`system:subscription:<name>`, state in `state.KV`). Followed closely.
- `internal/reconclient` Differ with a pluggable `Local`: the ledger becomes
the local side, so the diff compares the source with what was last pulled.
- `internal/syncmerge`: field typing (`FieldsFor`), canonical values and the
comparison form. The three-way rule is the same; the base is stored as digests
of the comparison form.
- `ingestspec` was considered and rejected: its mapping is string
interpolation for untyped inbound records; a pull maps typed fields one to one,
and interpolation would turn every value into a string.
- External refs (`external_ref`, unique per system): the durable link from a
local entity to its source row; a create that lost its ledger entry is recovered
by the uniqueness check.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**

`peers.yaml`:

```yaml
peers:
  catalogue:
    url: https://catalogue.example.com
    credential: peer_credential.catalogue
    every: 1h              # optional; without it only `rela peer pull` runs it
    on_gone: detach        # detach (default) | delete
    max_gone: 10           # default 10
    max_rows: 100000       # rows in scope, default 100000
    max_fetch: 10000       # rows fetched per run, default 10000
    entities:
      requirement:         # source type
        type: requirement  # local type, default the same name
        ref: catalogue_ref # local external_ref property; its system belongs to this peer
        fields:            # local field: source field; the source owns these
          title: title
          content: content
    relations:
      refines: {type: refines}
```

Ledger (`state.KV`, key `peersync/<name>/ledger.json`): per source element the
kind, source type, address, source view hash, the local id or relation key, a
state (linked, rejected, dropped) and the base digest per owned field. A config
hash; a changed mapping clears every hash so the next run refetches everything.
The ledger implements `reconclient.Local` (Sum/List over its rows), so the
Differ reports OnlyRemote (new), Differing (changed) and OnlyLocal (gone).

Run:

1. Load ledger; credential from the peer's secret; `GET _schema` for plurals.
2. Diff (bounded by max_rows and the Differ budgets).
3. Gone guard before any write: refuse when gone linked rows > max_gone
(or `--allow-gone`), or when a type with linked rows has no source rows left.
Rows of a type the source refused (Skipped) are never gone.
4. Entities: fetch `GET /api/v1/{plural}/{id}` (capped), check id/type,
build theirs from `fields:` (redacted = not reported, absent = cleared), coerce
with `syncmerge.Canonical`. Linked row: read ours through the gated reader,
three-way per field on digests: unchanged, write theirs, local divergence kept,
or conflict (row stops, reported, base kept). New row: adopt an entity already
holding the ref, else create with a generated id and the ref
(`WriteExternalRefs`). Validation errors reject the row.
5. Relations: endpoints mapped through linked ledger rows; missing endpoint
leaves the row for the next run. Create, or update owned meta. Fetch only when
the relation has owned fields.
6. Gone: relations first, then entities. detach unsets the ref and forgets
the row; delete deletes only when owned fields are unchanged locally and the
entity has no relations left, otherwise detaches and reports why.
7. Save the ledger (also after a failure, with what was applied).

The caller stamps `system:peer:<name>` (tool `peer-pull`) and binds it once: the
scheduler as for subscriptions, the CLI through the same binder.

Alternatives rejected: base in the `sync/<peer>` version tag (needs history, so
fs projects could not pull; the ledger also needs the source hash, which a tag
cannot hold); remote ids as local ids (the source would pick local ids); diff
against the local graph (local ids and mapping differ, so nothing would ever
match).

**Files to modify:**

- new `internal/peersync/` (config.go, ledger.go, run.go, fetch.go, report.go, tests)
- `internal/reconclient/httpremote/remote.go` (Get)
- `internal/syncmerge/syncmerge.go` (export the comparison key)
- `internal/principal/principal.go` (`system:peer:` prefix, tool)
- `internal/scheduler/` (peers tasks, job kind, deps)
- `internal/cli/peer.go`, `cli_wiring.go`, `validate.go`; `internal/projectsetup/validate.go`
- `.go-arch-lint.yml`, archguard allowlists if needed
- `internal/dataentry/jwtgate.go` godoc (#2158), docs-project guides

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**

- `peers.yaml`: operator-authored, checked at load (KnownFields, schema
checks). Credential name must start with `peer_credential.`.
- Source answers (untrusted): reconcile answers are validated by the Differ;
fetched rows are capped (4 MiB entity, 1 MiB relation, 8 MiB schema), must name
the requested id and type, only mapped fields are read, values are coerced to
the local field type, and the manager validates the write. The local id is
always generated locally. External ref ids are validated by
`metamodel.ParseExternalRef`. Error text from the source is sanitized.
- Ledger: own state; decoded strictly, a corrupt ledger fails the run.

**Security-Sensitive Operations:**

- Credential: read from secrets at run time, sent only to the configured
origin (no redirects), never logged or printed.
- Writes: only through `entitymanager` as `system:peer:<name>`, which grants
nothing by itself; with an `acl.yaml` the operator grants it a role. External
refs are written only by this path.
- Mass delete (issue #2119 note): detach default, max_gone, empty-type
guard, 401/403 is a failed run, deletes logged with run id and audited as
`system:peer:<name>`.
- No HTTP inside a store Tx: fetch, then write.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:**

- AC1-3: `TestPeerPull_EndToEnd` (dataentry): source router with peer key and
ACL; target memstore manager; pull, change source, pull again, local edit
conflict.
- AC4: `peersync` config table tests; `rela validate` reports peers errors.
- AC5-7: `peersync` run tests over a fake HTTP source: gone guard, allow-gone,
empty type, 401, hostile answers (wrong id, oversize body, bad enum value).
- Ledger as Local: Sum/List agree with a reference over random rows.
- CLI: `rela peer pull --dry-run` output and JSON.
- Scheduler: peers tasks load, name clash, minimum interval.

**Edge Cases:**

- Ledger missing (first run), corrupt, or written with another config.
- Entity created but ledger not saved: adopted through its external ref.
- Local entity deleted by a user: row dropped, not recreated.
- Field redacted at the source: not reported, never cleared.
- Relation whose endpoint is not pulled yet: retried next run.
- Source type skipped by the source policy: its rows are never gone.

**Negative Tests:**

- 401/403, 404 on fetch, oversize body, wrong id, type mismatch, invalid
enum, remote ref of 300 bytes, empty source scope.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**

- Mass delete or detach after a source ACL change: gone guard (above).
- Ledger and graph drift after a crash: external ref uniqueness and
idempotent reapply; a lost update re-runs because the hash is unchanged.
- Large ledgers: max_rows caps it; one KV document per peer.
- Archplan ratchet: new leaf-ish package with consumer-side interfaces; check
with `archplan check` before PR.

Effort: xl.

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**

- docs-project guide for peers.yaml (sync guide section) and CLI reference
(`rela peer pull`).
- GUIDE-acl-security: threat model for the pull side; fix #2157.
- `jwtgate.go` godoc (#2158).

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** the review-responses linked to TKT-I8L1EQU. The plan
changes they caused:

- One run per peer at a time, across processes: the CLI and the scheduler
both create the run in `schedulerstate` for task `peer:<name>` (`ErrRunActive`
refuses the second). The run deadline stays under the queue's 15 minute handler
cap.
- Ours is read raw (a write-prep read, as ingest does); the manager still
authorizes the write as `system:peer:<name>`. Every update is `PatchEntity` with
`ExpectedVersion` from that read and only the changed owned fields; a version
conflict re-reads and merges once, then reports.
- Detach keeps the external ref and keeps the ledger row as `detached`,
outside the diff. When the source lists the row again it is linked again with
its old base. `max_gone` counts entity and relation rows and applies to detach
and delete.
- A linked row whose local id is missing is looked up by its external ref
(a local rename) before it is treated as deleted.
- `fields:` refuses external_ref, file, computed and roll-up properties and
the ref property itself (syncmerge.FieldsFor plus a check).
- Relations get their own `fields:` (owned meta). An existing local relation
with the same key is adopted. At load the relation's from and to types must
admit the mapped local types.
- `_redacted` decides cleared versus hidden: absent and not listed means
cleared; with no `_redacted` key at all, absent means not reported.
- No base (new mapping, adoption): equal values link, different values are a
conflict.
- Source plurals from `_schema` must pass the type name grammar and are path
escaped. Skipped holds relation types only; a hidden entity type shows up as
zero rows and the empty-type guard covers it.
- Adoption can take over an entity an operator script tagged with the same
system; documented, and a differing adopted row is a conflict.
- `rela peer pull` runs the HTTP inline: it is an operator command, not a
write path, so `.claude/rules/jobs.md` does not apply; the scheduled path runs
as a job.
