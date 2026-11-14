---
id: PLAN-9S31UVT
type: planning-checklist
title: 'Planning: Declarative mapped import: rela import --mapping (JSON/XML/YAML/CSV, diff, Lua hook)'
started: "2026-11-14"
completed: "2026-11-14"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:**

In: package `internal/mapimport` (mapping spec + validation, decoders for JSON,
YAML, XML and CSV into one node tree, a small path language, plan/diff, apply
through the manager, pure Lua hook); `rela import --mapping <name> <file>` with
`--dry-run`, `--skip-errors` and `-o json`; `rela validate` checks
`imports/*.yaml`; OSCAL example in testdata; guide in docs-project.

Out: data-entry and MCP surfaces; faced entity types (refused at load); relation
properties; deleting entities that left the source (reported only); scheduled
imports; fetching over HTTP.

**Acceptance Criteria:**
1. JSON and XML sources map to entities with typed properties (test: JSON and XML catalogs).
2. Relations resolve by target key, including entities created in the same run (test).
3. A second run of the same import plans no changes (test).
4. `--dry-run` writes nothing (test: store unchanged, audit empty).
5. A Lua hook may change or skip a record and has no rela or io bindings (test).
6. A principal the ACL denies gets per-item failures, nothing written (test).
7. A DOCTYPE entity, an external entity and an oversized document are refused (test).
8. The OSCAL example mapping imports the example catalog (test).

## Research

- [x] For larger features: run `/research` to create a structured research doc
- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** RES-KEOOOTQ

**Existing Solutions:**
- `internal/importer` (native shape, writes the store directly): kept for the native format;
`--mapping` selects the new engine in the same command.
- `internal/ingest`/`ingestspec`: per-record, no diff, no relations; not reused for bulk import.
- `internal/safexml`: reused for XML. `lua.NewSandbox`, `lua.GoToLuaValue`, `lua.ToGo`: reused
for the hook. `external_ref` + `WriteExternalRefs`: usable as the key.
- JSONPath, XPath, jq/gojq, JMESPath considered; one small path language over a normalized tree
chosen (see research).

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**
1. Parse `imports/<name>.yaml` (KnownFields), validate against the metamodel: types exist and
are faceless, properties exist and are not computed/rollup, key property exists,
relation types allow the from/to types, paths compile, hook script path is
project-relative.
2. Decode the input under caps into `Node` (object/array/scalar with parent links).
3. For each entity mapping: select records; per record derive key, id, properties, content and
relation target keys; run the hook; collect per-record errors with the record
location.
4. Load raw stored rows once per type (`ListEntities`) and once per relation type
(`ListRelations`), as a write-prep read; gate every row before the plan names it
(`GetAddress` on the visible reader). Match by scope + key (or id); compute
creates, per-property updates, removals only for values with `clear_missing:
true`, relation adds, stale relations (removed only with `prune: true`), missing
entities (never deleted).
5. Apply: creates (`CreateEntity`, `WriteExternalRefs`), updates (`PatchEntity` naming only the
mapped properties), relation adds and removes through the manager. Per-item
failures are reported; apply refuses a plan with record errors unless
`--skip-errors`.

**Files to modify:**
- new `internal/mapimport/*`
- `internal/cli/import.go` (flag), new `internal/cli/import_mapped.go` (plan and report)
- `internal/projectsetup/validate.go` and the `rela validate` printer
- `.go-arch-lint.yml`, `.testcoverage.yml`
- `docs-project/entities/guides/GUIDE-mapped-import.md`, CLI reference guide, generated docs

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**
- Mapping file: operator-authored and trusted, but validated strictly at load (unknown keys,
types, properties, paths) so a typo fails loudly.
- Input file: untrusted. Byte cap (default 10 MiB, max 50 MiB), node and depth caps for every
format, safexml for XML (no DTD processing, no external entities,
element/attr/depth caps, charset allowlist), YAML alias expansion counted
against the node cap, record cap.
- Values: coerced to the property type; ids validated with `entity.ValidateID`; the manager
validates every write.

**Security-Sensitive Operations:**
- Writes go only through `entitymanager.Manager` as the CLI principal: ACL, field gate,
validation and audit apply. The diff reads raw rows; each row is gated with the
visible reader before the plan names it, and a key held by a hidden row is a
record error that does not name the row (the accepted one-bit channel, as in
remotesync).
- Hook: `lua.NewSandbox` (no io/os/debug/load, no rela bindings, no http/ai/mail), bound to the
command context; pure transform, so a dry run stays honest.
- No external commands, no network.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:** Table-driven tests in `internal/mapimport` on a
memstore-backed manager: JSON, XML, YAML, CSV import; relation by key to new and
existing targets; re-import no-op; dry run writes nothing; update reports
per-property changes and preserves unmapped properties; hook modifies and skips;
ACL-denied writes; malicious XML; OSCAL example. CLI test for `rela import
--mapping` text and JSON output. Validation tests for bad mappings.

**Edge Cases:**
- Path with no match (property absent, relation skipped or error), several matches on a
non-list property (error unless `join`), duplicate keys in source or store,
empty key.
- Unicode and NUL in values; numbers as keys; XML namespaces and mixed content.

**Negative Tests:**
- Oversized input, deep nesting, DOCTYPE entity, unknown mapping keys, unknown types or
properties, faced types, bad relation endpoint types, hook errors, hook
returning a non-table.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**
- Diff noise from value normalization (numbers, dates, trailing newlines): normalize both sides
before comparing; idempotence test pins it.
- Non-atomic apply: a partial failure leaves a partial import; re-running converges. Documented.
- Scope too wide when two catalogs share a type: `scope:` constant properties narrow matching
and the missing report.

Effort: l.

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**
- [x] docs/cli-reference.md - `rela import --mapping`
- [x] New guide: mapped import (with OSCAL example)

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** see review-response entities linked to TKT-K7HR49Y.
