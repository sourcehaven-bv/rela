---
id: PLAN-OI82FKO
type: planning-checklist
title: 'Planning: rela test v1 (3/4): rela test command, rela.test Lua module, seeded fakers and principals'
started: "2026-11-13"
completed: "2026-11-13"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:**

Part 3 of TKT-0MCLX9S. The full design is in PLAN-G8YMBDB. Parts 1 (NewInMemory)
and 2 (clock, mail, HTTP, AI and output-dir seams) are done.

In scope:
- `internal/apptest`: runner, per-case world, the `rela.test` Lua module, seeded schema-aware fakers, report.
- Principals: `t.as(user)` with roles from the real policy; `t.principal{roles}` with a per-case policy copy.
- `principal.ToolRelaTest`.
- `accessExpectation` moves from `internal/cli` to `internal/aclmap`.
- `rela test [paths...] --run --seed -o json`, outside `requiresProject`; refuses config-in-rela.db projects.

Out of scope (part 4, TKT-E2IMTMG): `tickets/tests/`, the CI step that runs it,
the guide and CLI reference.

**Acceptance Criteria:** AC1-9 and AC11-16 of PLAN-G8YMBDB. AC10 belongs to part
4.

## Research

- [x] ~~For larger features: run `/research`~~ (N/A: research done on the umbrella, PLAN-G8YMBDB)
- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A (see PLAN-G8YMBDB)

**Existing Solutions:** PLAN-G8YMBDB lists them. Used here:
`appbuild.NewInMemory` and its seams, `fsimport.Copy`,
`appbuild.IdentityBinder`, `aclmap.Engine.MapPrincipal`, `mail.MemorySender`,
the `internal/lua` sandbox and require, gofakeit for names and sentences.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:** as PLAN-G8YMBDB, with these refinements found while
reading the code:
- **The world is built lazily.** `t.fixtures`, `t.principal` and `t.clock` record setup. The first write or read builds the world with `NewInMemory`, loads the fixtures with `fsimport.Copy`, and creates the principals' user entities. `t.principal` after that point is an error that says to declare principals first. This is needed because `WithACL` needs a store-backed graph, which does not exist before the world.
- **The policy copy enters through a new option `appbuild.WithPolicyAmendment(func(*acl.Policy) error)`.** It runs on the policy `prepare` just parsed from acl.yaml, before validation. Each `prepare` parses its own copy, so the shared policy is never touched. The test world adds one `assignments` entry per synthetic principal.
- **Several roles per principal.** `assignments` maps a member to one role. The first role is assigned to the user. Each further role is assigned to a group entity (of the user entity type) that the user is linked to by the membership relation, written as given state. A project with no `user_entity_type` supports one role per principal, and the error says so.
- **`asserted_role_assignments` is not used.** `acl map` reports asserted grants for every principal, so `t.expect_access` would see them for everyone.
- **The test VM** is a gopher-lua state built by a new `lua.NewSandbox`, which opens the same safe library set and installs rela's `require` over `tests/` and `lib/`. `require("rela.test")` returns the module. One VM per file, so cases are closures; each case gets its own world.
- **Failure locations** come from `LState.Where(1)` at the assertion call.

Alternatives rejected: rebuilding the world when `t.principal` is called late
(loses the store), mutating the live policy's maps (data race and breaks the
never-mutate invariant), asserted roles (see above).

**Files to modify:** `internal/appbuild/seams.go` (WithPolicyAmendment),
`internal/appbuild/appbuild.go` (apply it), `internal/principal/principal.go`,
`internal/lua/sandbox.go` (new), `internal/aclmap/expect.go` (moved),
`internal/cli/acl_expect.go`, `internal/cli/acl_map.go`, `internal/cli/test.go`,
`internal/cli/kong.go`, `internal/apptest/*`, `internal/archguard` allowlists,
`.go-arch-lint.yml`, `go.mod`.

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:** test files, fixtures and `lib/` are
operator-authored, read through `rootfs` under the project root. A fixture path
outside the root is rejected. Principal names and roles exist only in the case's
memory; an undeclared role is an error.

**Security-Sensitive Operations:** no disk writes (MemFS, temp output dir
outside the root, AC6). No live store, locks or DSN (outside `requiresProject`).
Mail captured, HTTP and AI refused, no secrets (NewInMemory defaults). Identity
is bound through `appbuild.IdentityBinder`; `ToolRelaTest` is listed in the
`VerifiedFrom` exceptions.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:** Go tests in `internal/apptest` run `.test.lua` files against
a fixture project in `testdata/` and check the report: AC1 automation effect,
AC2 access positive and negative control, AC3 clock across consumers, AC4 mail
capture and recipient policy, AC5 HTTP/AI refusal, AC6 tree hash, AC7 isolation,
AC8 location and exit code, AC9 sandbox, AC11 generated create across 200 seeds
with zero warnings, AC12 seed reproduction, AC13 seed-derived clock, AC15
missing config, AC16 goleak over 100 cases. AC14 is a sqlite-tagged CLI test.

**Edge Cases:** no `tests/` folder; a file with no cases; a Lua error outside
`t.case`; a missing fixture directory; a negative `t.advance`; `t.principal`
after the world exists; unicode names.

**Negative Tests:** failing assertion with file:line; real HTTP/AI call; file
writes from the VM; unknown type in `create`; undeclared role.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:** see PLAN-G8YMBDB. Added: group entities for extra roles show up in
`list` results; documented, and only created when a principal has more than one
role.

Effort: xl.

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:** the guide and CLI reference are part 4 (TKT-E2IMTMG).
This part writes package godoc and the command's help text.

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** the design was reviewed on the umbrella (18
review-responses on TKT-0MCLX9S, all addressed in PLAN-G8YMBDB). The refinements
above follow from reading the code and keep that design's invariants.
