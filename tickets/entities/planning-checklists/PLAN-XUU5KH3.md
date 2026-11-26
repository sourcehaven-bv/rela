---
id: PLAN-XUU5KH3
type: planning-checklist
title: 'Planning: Apply pending config migrations in memory on load and warn instead of refusing to boot'
started: "2026-11-25"
completed: "2026-11-25"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:**

In: schema.yaml (root file) and data-entry.yaml, at boot and on data-entry hot
reload. Pending per-file migrations are applied to the parsed YAML tree in
memory, the result is parsed by the normal strict parser, and a warning names
the file and migrations. `rela validate` stays strict.

Out: project-level migrations (`automations_to_flows`), which stay hard errors;
schema includes (never migration-checked today); acl.yaml and flows.yaml (their
loaders run no detection today); a health/admin surface for pending migrations
(follow-up); writing anything to disk.

**Acceptance Criteria:**
1. A schema.yaml with pre-migration syntax loads through `metamodel.FSLoader`
and yields the same metamodel as after `rela migrate`; a warning is logged.
Test: loader test with `allow_acl_bypass: true` and `cardinality` legacy keys.
2. A data-entry.yaml with pre-migration syntax loads through `loadConfig` at
boot and on reload; a warning is logged. Test: `loadConfig` test and a reload
test.
3. Files on disk are unchanged. Test: compare bytes after load.
4. `automations:` in schema.yaml still refuses with the migration error.
Test: existing tests keep passing.
5. `rela validate` still fails on deprecated syntax in both files. Test:
existing projectsetup validate tests, plus a strict-loader test.

## Research

- [x] ~~For larger features: run `/research` to create a structured research doc~~ (N/A: approach settled in discussion; small change on existing seams)
- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] ~~Looked for reference implementations in other projects~~ (N/A: reuses rela's own migration runner)
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A

**Existing Solutions:**
- `migration.ApplyWithMetamodel` (internal/migration/runner.go:104) already
applies migrations to a yaml.Node; the new in-memory variant shares its loop.
- `migration.Advisory` / `migration.Blocking` (migration.go:105-140) are the
existing "old syntax still loads" escape hatch.
- `projectsetup.migrationSchema` adapts a metamodel for MetamodelAware
migrations; it moves to `internal/metamodel` so dataentry can use it.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**
1. `migration.MigrateBytes(data, ft, meta) (InMemory, error)`: parse, detect,
split detections into project-level (returned as `Blocking`) and advisory
(skipped, as today), apply the rest to the tree, re-marshal. An Apply error is
returned and refuses the load (fail closed).
2. `metamodel.FSLoader.Load`: read the root file, run `MigrateBytes`; blocking
detections return `*migration.Error` as today; applied ones log `slog.Warn` and
the migrated bytes are parsed via a new `loadData(path, data, fs)` split out of
`Load`. `NewStrictFSLoader` keeps the refusal and is used by `projectsetup`
validation.
3. `metamodel.MigrationSchema(m)` replaces `projectsetup.migrationSchema`.
4. `dataentry.loadConfig`: run `MigrateBytes` with `MigrationSchema(meta)`,
log the warning, parse the migrated bytes. Project-level detections still
refuse. `projectsetup.CheckDataEntry` stays strict.

Alternatives rejected: making parsers accept both spellings (widens privilege
fields such as `allow_acl_bypass`, and the parser would carry every legacy
form); writing migrated files at boot (the server must not edit operator files,
and may run read-only).

**Files to modify:** internal/migration/runner.go, internal/metamodel/
loader.go, loader_service.go, migrationschema.go (new), internal/projectsetup/
migrate.go, migrationschema.go (removed), validate.go,
internal/dataentry/app.go, tests alongside, docs.

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:** Operator-authored schema.yaml and
data-entry.yaml (trusted config, not secret). The migrated tree goes through the
same strict parser and validation as a file on disk, so nothing reaches the app
that `rela migrate` + load would not also produce.

**Security-Sensitive Operations:** Threat model: (a) a migration that changes a
privilege field. Only `acl-bypass-enum` does; it maps `true` to `read+write`,
the exact prior capability, so in-memory application grants nothing the operator
did not already grant. (b) A migration bug producing a wider config: same risk
as `rela migrate` today; the output is parsed and validated strictly. (c) An
Apply error: refuses the load, never falls back to partial migration. (d)
Hot-reload frame: the deprecated-syntax error is no longer shown for applied
migrations; the path-stripping in `publicConfigErrorMessage` still covers the
blocking case. (e) Silent drift: `rela validate` stays strict so CI keeps
flagging deprecated syntax; the server logs a warning on every load.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:** see acceptance criteria 1-5.

**Edge Cases:** no detections (bytes returned untouched, no warning); only
advisory detections (no warning, no rewrite); invalid YAML (strict parser error
as today); mix of applied and project-level detections (refuses).

**Negative Tests:** Apply error refuses load; strict loader refuses;
`automations:` refuses.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:** yaml.Node re-marshal changes formatting: irrelevant, bytes are only
parsed. Line numbers in later parse errors refer to migrated text: the warning
names the migrations, and `rela migrate` fixes the file. Effort: m.

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:** docs-project guide on upgrading / `rela migrate`
(deprecated syntax now warns at boot instead of refusing).

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** see review-responses linked to TKT-3JF7FSN.
