---
id: PLAN-RPDQ8K7
type: planning-checklist
title: 'Planning: rela validate accepts deprecated link: true that rela-server refuses'
started: "2026-10-30"
completed: "2026-10-30"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:**

rela-server refuses a data-entry.yaml that still has deprecated syntax:
`loadConfig` (internal/dataentry/app.go:1587) runs `migration.DetectBytes(...,
FileTypeDataEntry)` and returns a `*migration.Error`. `rela validate` goes
through `projectsetup.CheckDataEntry`, which never runs that detection, so it
reports the file as valid. `link: true` is one such syntax (`link-to-string`
migration); the data-entry guide still uses it in 12 examples and documents
`link` as a bool.

In:
- `CheckDataEntry` runs the same migration detection first and returns the same `*migration.Error` ("uses deprecated syntax ... Run 'rela migrate'"). This covers `rela validate` and `rela-server configure`, which both call it.
- GUIDE-data-entry: every `link: true` example becomes `link: detail`; the column and section option tables document `link` as a string (`detail` or `document/<name>`); the best-practices item says `link: detail`. Regenerate docs/.

Out: migration detection for acl.yaml (the server does not refuse it either);
changing which syntaxes are deprecated.

**Acceptance Criteria:**

1. `rela validate` on a project whose data-entry.yaml has a list column `link: true` reports an error naming the deprecated syntax and `rela migrate`, and exits non-zero (projectsetup test via `ValidateWithFS`).
2. The same project with `link: detail` validates clean (existing valid-project test plus a `link: detail` case).
3. The error is a `*migration.Error` (`migration.IsMigrationError`), the same type the server returns.
4. No `link: true` remains in GUIDE-data-entry.md and docs/data-entry.md after `just docs`.

## Research

- [x] ~~For larger features: run `/research` to create a structured research doc~~ (N/A: effort s)
- [x] ~~Searched for existing libraries that solve this problem~~ (N/A: in-repo check)
- [x] Checked codebase for similar patterns or reusable code
- [x] ~~Looked for reference implementations in other projects~~ (N/A)
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A

**Existing Solutions:** `loadConfig` in internal/dataentry/app.go:1583-1593
(server-side detection); `migration.Error` in internal/migration/runner.go:218;
the metamodel loader does the same for schema.yaml
(internal/metamodel/loader_service.go:58), which is why validate already catches
deprecated schema syntax.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**

At the top of `projectsetup.CheckDataEntry`, call `migration.DetectBytes(data,
migration.FileTypeDataEntry)`; when it finds anything, return
`&migration.Error{FilePath: dataentryconfig.ConfigFile, Detections: ...}`.
projectsetup already imports migration (migrate.go), so no new package edge.

`rela-server configure` (cmd/rela-server/configure.go `Validate`) unwraps a
`*migration.Error` and adds one problem per detection: `data-entry.yaml:
deprecated syntax: <description>; run rela migrate`. Without this it would print
the file name twice in one multi-line problem (RR-PT0E1VD).

Alternative rejected: making `loadConfig` call `CheckDataEntry`. dataentry would
import projectsetup, and loadConfig has extra steps (normalization, warnings)
that validate does not need. One shared call to `migration.DetectBytes` is
enough to keep them aligned.

**Files to modify:**
- internal/projectsetup/validate.go
- internal/projectsetup/validate_test.go
- cmd/rela-server/configure.go (+ test)
- docs-project/entities/guides/GUIDE-data-entry.md (+ generated docs/data-entry.md)

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:** data-entry.yaml, operator-authored config.
Detection is read-only YAML node inspection already used by the server.

**Security-Sensitive Operations:** None. The error names the config file and the
migration descriptions; config is not a secret.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:** table-driven `TestValidateWithFS_DeprecatedDataEntry` over a
memory FS: column `link: true` (error, IsMigrationError, message mentions `rela
migrate`, `HasErrors()` true and `DataEntryValid` false so the CLI exits
non-zero), section-level `link: true` in a view (error), `link: detail` (valid).
Manual: build rela, run `rela validate` on a temp project with `link: true`.

**Edge Cases:** unparseable YAML still reports the parse error (DetectBytes
returns nothing for invalid YAML); empty file stays valid.

**Negative Tests:** the `link: true` cases above.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:** a project that passed `rela validate` now fails it. That is intended:
rela-server already refuses it. `rela-server configure` also refuses it now,
consistent with startup. Effort: s.

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**
- [x] docs/data-entry.md - examples and option tables (via GUIDE-data-entry)

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** RR-PT0E1VD (minor, addressed), RR-8MWKU55 (nit,
addressed). No critical or significant findings.
