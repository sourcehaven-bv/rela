---
id: PLAN-9JWPUZ
type: planning-checklist
title: 'Planning: Relation-based validation gates are silently dropped; port workflow gates to Lua + enforce done-before-PR'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:** IN: (a) make the 14 relation-based workflow gates actually fire; (b)
enforce done-before-PR via the `/pr` command and the `rela-tickets` CI job; (c)
resolve the historical debt these gates surface. OUT: native engine support for
a `relations:` block on `ValidationRule` (tracked as a follow-up ticket); a
time/date property on tickets.

**Acceptance Criteria:**
1. A `done` ticket lacking a completed review checklist fails `rela validate`
(exit 1). Test: run `rela validate --check validations` — verified it flags the
gap and passes once a `has-review -> done` checklist exists.
2. `/pr` refuses unless the ticket is `done` + validate-clean.
3. CI fails a PR whose touched ticket is left in `ready`; chore/* is validated
(no longer fully exempt).

## Research

- [x] ~~run /research~~ (N/A: root cause already isolated empirically in-session)
- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A (small, well-understood change)

**Existing Solutions:**
- Root cause found: `ValidationRule` (internal/metamodel/types.go:50) has no
`relations` field, so the `relations:` block is dropped at YAML parse time — all
14 gates are inert. Proven with a probe rule flagging 32 offenders.
- Reusable pattern: the engine DOES run `lua_file:` rules
(validate-justification.lua is a live example). `rela.get_relations{}` +
`rela.get_entity()` provide the needed read API.
- FEAT-GM14 added `content.checklist` validation on the same rule surface — this
ticket extends it with relation cardinality.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:** Port the 14 gates to one parameterized
`require-relation-count.lua` (relation-type, min/max, threshold, target
where-filters). The rule's native `when:` still selects applicability; the Lua
only counts matching relations. Enforce done-before-PR in
`.claude/commands/pr.md` (gate step) and the `rela-tickets` CI job (validate on
chore/* + a done-before-merge status check).

**Alternatives considered:**
- Native `relations:` engine support — deferred (larger Go change; follow-up).
- Port to Lua — chosen (works today, no engine risk).
- Fail-loud loader only — insufficient (leaves gates non-functional).

**Files to modify:**
- tickets/validations/require-relation-count.lua (new)
- tickets/metamodel.yaml (14 rules)
- .claude/commands/pr.md
- .github/workflows/ci.yml

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**
- Lua validator args come from the trusted metamodel (not user input); still
guarded: missing/invalid relation-type, mode, threshold, and malformed key=value
filters each return an explicit violation message (fail-loud).
- CI job: `github.head_ref` is read via an `env:` var and only prefix-matched
against literals — no interpolation into a `run:` command (injection-safe).

**Security-Sensitive Operations:**
- CI branch-prefix logic gates enforcement; kept minimal and literal.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:**
- Gate fires: `rela validate` flags a done ticket with no completed review
checklist (verified: 31/27/83/7 across the four gates before backfill).
- Gate passes: a backfilled ticket with `has-review -> done` passes.
- CI status parse: awk reads quoted and unquoted `status:` correctly (verified).

**Edge Cases:**
- min:1 with no where-filter (any-status child) vs. with where=status=done.
- max:0 response gates (must stay at 0 violations — verified).
- Quoted vs unquoted YAML frontmatter status values.

**Negative Tests:**
- Malformed lua_args → explicit violation message rather than silent pass.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**
- Enabling gates surfaces 129 historical offenders → mitigated by a deliberate
prune (114 pre-July, cascade to ~1016 entities) + backfill (15 July), each in
its own commit, git-reversible. Verified 0 kept entities lose a required link.
- Effort: m.

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**
- [x] N/A - Internal tooling/process change; the gate mechanism is documented
inline in metamodel.yaml and the Lua header. No user-facing product docs.

## Design Review

- [x] ~~Run /design-review~~ (N/A: approach reviewed interactively with the maintainer in-session)
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** N/A (interactive review; no RR entities)
