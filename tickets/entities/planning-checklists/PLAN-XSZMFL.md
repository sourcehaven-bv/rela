---
id: PLAN-XSZMFL
type: planning-checklist
title: 'Planning: ACL doc-fields: RoleDef.description + top-level policy description (rela-docs phase 1b)'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:**

IN:
- `RoleDef.Description string` (`yaml:"description,omitempty"`) in `internal/acl/policy.go`.
- `Policy.Description string` (`yaml:"description,omitempty"`) in the same file, plus `"description"` added to `knownPolicyKeys` so the tolerant loader does not warn on it.
- An in-tree example `acl.yaml` populated with a policy-level description and per-role descriptions.
- Tests: parse, absence, round-trip, and unknown-key-warning suppression.

OUT:
- Any wire/API exposure of roles. No dataentry endpoint reads `RoleDef` today (grepped: `RoleDef`/`.Roles` appear only in `internal/acl`). The phase-2 generator reads `Policy` directly, so no serialization type is added now — mirrors TKT-0YBFT8's deliberate omission of doc-fields from the v1 CustomType wire.
- The generator itself (phase 2).
- Any ACL behavior change. Descriptions are prose; the authz path never reads them; `Validate()` is untouched.

**Acceptance Criteria:**
1. `RoleDef.description` parses and round-trips (marshal → unmarshal preserves it).
2. `Policy` top-level `description` parses and round-trips.
3. The new top-level `description` key does NOT emit the loader's "unknown key" `slog.Warn` (it is in `knownPolicyKeys`).
4. Backward-compat: a policy without either field loads identically to before; `Validate()` result unchanged.
5. Example `acl.yaml` populated; loads clean (no warnings, `Validate()` ok).
6. Tests cover each field + the unknown-key suppression.

## Research

- [x] ~~For larger features: run `/research`~~ (N/A: the arc's research RES-EK7LSA already covers doc-field additions; this is a mechanical mirror of phase 1a)
- [x] Searched for existing libraries — N/A, pure struct-tag additions
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** RES-EK7LSA (arc research; motivates the ACL role-description addition for the operator capability-matrix section).

**Existing Solutions:**
- Direct template: **TKT-0YBFT8** (phase 1a) added `Metamodel.Description`, `CustomType.Descriptions`, `TransitionDef.Help` with the identical additive/optional/round-trip shape. This ticket is the ACL-side counterpart.
- The metamodel loader gotcha from 1a (`validTopLevelKeys` had to gain `description`, else the parser rejected it) has an ACL analogue: `knownPolicyKeys` (`internal/acl/policy.go:304`). BUT the ACL loader is **tolerant** — unknown keys warn-and-continue (`LoadPolicy`, policy.go:338-343), they do not hard-reject. So omitting the allowlist entry would only produce a spurious warning, never a load failure. We still add it (AC3) for clean output.
- `RoleDef` (policy.go:166) is a plain struct decoded as a map value with no `KnownFields`/allowlist gate on its own keys, so the field needs only the yaml tag.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified

**Technical Approach:**
1. Add `Description string` with tag `yaml:"description,omitempty"` as the first field of `RoleDef` (policy.go:166).
2. Add `Description string` with tag `yaml:"description,omitempty"` to `Policy` (policy.go:87) and `"description": true` to `knownPolicyKeys` (policy.go:304).
3. Add an example `acl.yaml` under the phase-1a prototype project (`prototypes/data-entry/project/acl.yaml`) — the same project whose `metamodel.yaml` got the phase-1a descriptions — carrying a top-level `description:` and a `description:` on each role. This is the corpus the phase-2 generator demo will run against.
4. Tests in a new `internal/acl/docfields_test.go` mirroring `internal/metamodel/docfields_test.go`.

**Files to modify:**
- `internal/acl/policy.go` — the two struct fields + `knownPolicyKeys` entry.
- `internal/acl/docfields_test.go` (new) — the tests.
- `prototypes/data-entry/project/acl.yaml` (new) — example policy with descriptions.

## Security Considerations

- [x] Input sources identified
- [x] Input validation approach defined
- [x] Security-sensitive operations identified
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**
- Input is `acl.yaml`, operator-controlled config (same trust level as the rest of the policy). The new fields are free-text prose consumed only by the (future) doc generator; they are never evaluated, never reach the authz decision path, and are not interpolated anywhere. No validation needed beyond YAML string decoding.

**Security-Sensitive Operations:**
- None. Explicitly NOT touched: `Policy.Validate()` (the security-critical invariants — read⊇update/delete coverage, membership-relation blank-guard, delegate-X gate) is unchanged. Adding prose fields cannot widen a grant.
- Note for phase 2 (not this ticket): the generator must treat these descriptions as untrusted text when it renders them (esp. into Markdown/HTML/mermaid) — carry forward the phase-1a mermaid-injection lesson. Out of scope here; flagged for the phase-2 ticket.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined
- [x] Integration test approach defined

**Test Scenarios:**
- AC1/AC2 — `TestLoadPolicyBytes_DocFields_Present`: a policy with top-level `description` and a role `description` decodes with both populated.
- AC1/AC2 round-trip — `TestPolicyDocFields_RoundTrip`: `yaml.Marshal` then `yaml.Unmarshal` preserves both.
- AC3 — `TestLoadPolicy_DescriptionKeyNotWarned`: install a `slog` handler capturing records, `LoadPolicy` a temp file with top-level `description`, assert no "unknown key" record for `description`. (Sanity-negative: an actual unknown key like `bogus:` still warns.)
- AC4 — `TestLoadPolicyBytes_DocFields_Absent`: a policy without either field loads with empty descriptions and `Validate()` == nil, identical to today.
- AC5 — `TestExampleACLPolicyLoads` (or a loader smoke over the prototype file): the prototype `acl.yaml` loads clean.

**Edge Cases:**
- Empty/missing description → empty string, omitted on marshal (`omitempty`). Verified by AC4.
- A role with no description among roles that have one → only the annotated role carries prose. Covered in the present-case fixture (multiple roles, one without).
- Unicode / multiline prose in a description → plain YAML string; no special handling. Not separately asserted (stdlib YAML behavior).

**Negative Tests:**
- AC3 sanity-negative: an unknown key other than `description` still produces the warning (guards against accidentally suppressing all warnings).

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**
- Practically none. Additive optional fields, tolerant loader, no authz-path touch. Lower risk than phase 1a (which touched a hard-rejecting loader). Mitigation is the round-trip + absence + Validate-unchanged tests.
- Effort: **s** (a hair above xs only because of the example `acl.yaml` + the slog-capture test for AC3).

## Documentation Planning

- [x] User-facing docs identified
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**
- [x] `docs/acl-overview.md` (source: `docs-project/entities/guides/GUIDE-acl-overview.md`) — document the two optional `description` fields in the policy reference. Regenerate via `just docs` (same generated-doc pipeline as phase 1a's metamodel guide — edit the SOURCE guide, not `docs/`).
- [x] ~~Others~~ (N/A: only the ACL guide is affected)

## Design Review

- [x] ~~Run `/design-review`~~ (N/A: mechanical mirror of the already-reviewed TKT-0YBFT8 design; no new design decisions. Approach + security reasoning documented above.)
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** N/A (none; see above)
