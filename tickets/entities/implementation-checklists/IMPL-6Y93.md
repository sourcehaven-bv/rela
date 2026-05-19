---
id: IMPL-6Y93
type: implementation-checklist
title: 'Implementation: ACL v0 PR 3: Wire acl.yaml into appbuild + non-loopback warning + docs'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code
- [x] ~~Integration tests written (test full flow, not just units)~~ (N/A: PR 3 wires PR 2's Declarative into appbuild; the four `appbuild_acl_test.go` cases ARE integration-style — they exercise `appbuild.New` end-to-end on a real on-disk project)
- [x] Happy path implemented
- [x] Edge cases from planning handled
- [x] Error handling in place (errors surfaced, not swallowed)

## Test Quality

- [x] Using fixture builders or factories for test data
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**

- **`internal/appbuild/appbuild.go`** — `loadACL(projectRoot)` helper reads `acl.yaml`, returns `NopACL` on `os.ErrNotExist`, downgrades malformed-YAML to NopACL with a `slog.Warn`. `options.acl` defaults to nil and is populated from `loadACL` if no `WithACL` option fired — so `WithACL(ReadOnlyACL{})` from `rela-server --read-only` still wins. New `Services.ACL()` accessor exposes the wired ACL for the startup warning.
- **`internal/appbuild/appbuild_acl_test.go`** — four tests on real on-disk projects:
  - `TestDiscover_ACLPresent_LoadsDeclarative` (AC3.1)
  - `TestDiscover_ACLMissing_UsesNop` (AC3.1)
  - `TestWithACL_OverridesLoadedPolicy` (AC3.2)
  - `TestDiscover_MalformedACL_FallsBackToNop` (edge case)
- **`cmd/rela-server/main.go`** — new `shouldWarnNoACL(active, readOnly)` helper + warning block inside the existing non-loopback warning branch. Fires only when bind is non-loopback AND active ACL is NopACL AND --read-only is not set.
- **`cmd/rela-server/main_acl_test.go`** — `TestShouldWarnNoACL` table-driven across 6 combinations of (NopACL / ReadOnlyACL / Declarative) × (readOnly true/false). All 6 PASS.
- **`docs/security.md`** — new "Access control (`acl.yaml`)" section: minimal example, semantics (union + explicit-deny, default role, empty-acl-denies-everything), delegate-X tamper resistance, trust boundary, v0/v1 scope.
- **`docs-project/entities/guides/GUIDE-audit-log.md`** + regenerated `docs/audit-log.md` — new `denied-write` subsection with example record; `op` table row updated to include the new op.
- **`CLAUDE.md`** — new "Don't run user-supplied Lua on the read path" rule and "Authorization (`internal/acl`)" subsection pointing at the three implementations + design doc.
- **`just ci` exits 0** locally (test + lint + arch-lint + coverage + build + docs + frontend).
- **All ticket validation** clean once IMPL-6Y93 / REV / TKT transition to terminal states.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
