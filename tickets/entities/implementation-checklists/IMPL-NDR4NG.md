---
id: IMPL-NDR4NG
type: implementation-checklist
title: 'Implementation: ACL: dedicated authorization-misconfiguration validator / audit insights (escalation foot-guns, dead assignments, un-gated membership)'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code (24 in aclaudit, 4 CLI)
- [x] Integration tests written (CLI command end-to-end through real LoadPolicy + render + exit-code)
- [x] Happy path implemented
- [x] Edge cases from planning handled (nil meta; "*" sentinel; everyone read-only; whitespace)
- [x] Error handling in place (LoadPolicy errors surfaced; missing acl.yaml → exit 0)

## Test Quality

- [x] fakeMetamodel fixture for Tier-B; real temp-dir acl.yaml for CLI
- [x] No hardcoded values where an object is in scope (assert on Rule constants / Severity)
- [x] Only specifying values that matter; negative case per check
- [x] Golden clean-policy fixture (anti-false-positive guard)

## Manual Verification

- [x] Feature manually tested end-to-end (built `rela`, ran `acl audit` on a temp project)
- [x] Each acceptance criterion verified
- [x] Edge cases manually verified

**Verification Evidence:**

New code:
- `internal/aclaudit/` — `Audit(policy, MetamodelReader) []Finding`; `Finding{Rule,
Severity, Subject, Detail, Fix}` + 5-level `Severity`; `HasAtLeast`; narrow
`MetamodelReader` interface; `isPrivileged` (RR-LXI3NW); Tier A (A1, A1b, A2,
A3, A4, A5, A6, A7, A9, A10) in tier_a.go; Tier B (B1, B2, B3, B4, B5, B7) in
tier_b.go. Deterministic sort (severity, rule, subject).
- `internal/cli/acl.go` — `rela acl audit` (`--exit-code`, `-o json`); the
metamodel adapter over `*metamodel.Metamodel` (consumer side, keeps aclaudit's
dep to `[acl]`).
- `internal/cli/kong.go` — registered `ACL ACLCmd`; added `"acl"` to
`requiresProject`; bumped CLI `//plimsoll:max-fields` 38→39 (registry field).
- `.go-arch-lint.yml` — `aclaudit` component, `mayDependOn: [acl]`, cli→aclaudit.

Validate migration (TKT-Z8A62F follow-through):
- Removed `warnMembershipRelationHardening` + its call from `Policy.Validate`;
Validate is a pure structural gate again. Exported
`Policy.EffectiveMembershipRelation()` (resolver + audit share one source of
truth). Deleted the two membership warn-tests from policy_test.go; behaviour
re-asserted as aclaudit findings A1/A1b.

Manual smoke (temp project, intentionally misconfigured):
- `everyone: update` → A3 critical; un-gated member-of + assignment → A1 high;
`update: [tickets]` typo → B1 high. Sorted critical→high. `--exit-code` → rc 1.
- `-o json` → AnalysisResult envelope (status warning, count 3, details[]).
- Clean gated policy → "no findings", rc 0.

Gate results (all green):
- `go build ./...` — OK
- `go test -race ./internal/{acl,aclaudit,cli,appbuild,dataentry,mcp}/...` — ok
- `golangci-lint run` (aclaudit, acl, cli) — 0 issues
- `just arch-lint` — no warnings (aclaudit component added)
- `just plimsoll` — rc 0 (narrow MetamodelReader; CLI field pin bumped)
- `just coverage-check` — PASS (aclaudit 89.8%; total 76.9%)
- `just docs` — regenerated, idempotent; `rela acl audit` documented in
GUIDE-acl-security → docs/acl-security.md

Branch note: stacked on `feat/acl-configurable-membership-relation` (PR #1060),
because this ticket migrates the membership warns that only exist there. The PR
targets that branch (or develop once #1060 merges).

## Quality

- [x] Follows project patterns (Kong `DBCmd`/`AnalyzeCmd` model; `output.AnalysisResult`;
consumer-side interface per CLAUDE.md)
- [x] DRY — single `isPrivileged` / `EffectiveMembershipRelation`; shared sort helpers
- [x] No security issues introduced — audit is read-only, makes no auth decisions;
conservative gating prevents false-positive criticals (golden clean-policy test)
- [x] No silent failures (LoadPolicy errors returned)
- [x] No debug code left behind
