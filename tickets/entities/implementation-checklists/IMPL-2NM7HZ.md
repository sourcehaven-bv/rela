---
id: IMPL-2NM7HZ
type: implementation-checklist
title: 'Implementation: ACL doc-fields: RoleDef.description + top-level policy description (rela-docs phase 1b)'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code — `internal/acl/docfields_test.go` (5 tests)
- [x] Integration tests written — AC5 loads the real in-tree prototype `acl.yaml` end-to-end through `LoadPolicy` (file → typed Policy → Validate)
- [x] Happy path implemented — `RoleDef.Description` + `Policy.Description` parse; `knownPolicyKeys` entry silences the warning
- [x] Edge cases from planning handled — absent field stays empty (`viewer` role in the fixture has none); Validate() unaffected
- [x] Error handling in place — N/A: no new error paths; prose fields are plain string decode. `Validate()` untouched, so no new failure modes.

## Test Quality

- [x] Using fixture builders — shared `docFieldsPolicyYAML` const + the existing `writeTempPolicy` helper (mirrors `policy_test.go`)
- [x] No hardcoded values where object is in scope — round-trip test compares `p2.* == p1.*`, not literals
- [x] Only specifying values that matter — fixtures carry the minimum to exercise present/absent
- [x] Interpolated values from objects — round-trip asserts against `p1`
- [x] Property comparisons use original object — yes (round-trip)

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**

- `go test ./internal/acl/` — PASS (all package tests, incl. the new 5 + existing parity/round-trip/unknown-key).
- AC1/AC2 — `TestLoadPolicy_DocFields_Present`: PASS (both descriptions populated; viewer's absent stays empty).
- AC2 top-level — same test + `TestLoadPolicy_DescriptionKeyNotWarned`: PASS.
- AC3 — `TestLoadPolicy_DescriptionKeyNotWarned`: PASS. Confirms `description` is NOT warned AND a genuinely-unknown `bogus_key` still IS (warning path proven live, not globally suppressed).
- AC4 absence — `TestLoadPolicy_DocFields_Absent`: PASS (empty descriptions, `Validate()==nil`).
- AC4 round-trip — `TestPolicyDocFields_RoundTrip`: PASS (marshal→LoadPolicyBytes preserves both).
- AC5 — `TestLoadPolicy_PrototypeExample`: PASS; the in-tree `prototypes/data-entry/project/acl.yaml` loads clean with role+policy descriptions.
- AC6 — 5 tests cover each field + suppression.
- `go build ./...`, `just lint` (0 issues), `just arch-lint` (OK), `just lint-md` (0 issues), and `just docs` (regenerated `docs/acl-overview.md`) all green.
- Cross-package: `go test ./internal/dataentry/... ./internal/appbuild/...` PASS (the shared `Policy` struct change breaks nothing downstream).

## Quality

- [x] Code follows project patterns — direct mirror of TKT-0YBFT8 (phase 1a) additive doc-fields; test file mirrors `internal/metamodel/docfields_test.go` and `internal/acl/policy_test.go` conventions
- [x] Checked for DRY opportunities — none warranted; two one-line struct fields + one allowlist entry. No abstraction to extract.
- [x] No security issues introduced — prose fields never reach the authz path; `Policy.Validate()` (the security invariants) untouched; no interpolation. Phase-2 note: generator must treat descriptions as untrusted when rendering (flagged in planning, out of scope here).
- [x] No silent failures — no new error paths
- [x] No debug code left behind — the temporary smoke test was removed after verification
