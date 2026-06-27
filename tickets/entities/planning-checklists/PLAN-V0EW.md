---
id: PLAN-V0EW
type: planning-checklist
title: 'Planning: acl v1 wiring: appbuild + dataentry middleware + SSE audit + docs (PR 4)'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:** See TKT-YG35 body — exhaustive in/out, AC list, RR matrix.

**Acceptance Criteria:** All 7 criteria from TKT-YG35 are concrete + testable;
each names a test.

## Research

- [x] ~~`/research`~~ (N/A: reference branch `feat/acl-v1-tkt-svxl` is the research artefact)
- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] ~~external reference implementations~~ (N/A: internal wiring)
- [x] Reviewed relevant rela concepts for prior art

**Research findings (this PR's own investigations):**

1. **SSE audit (parallel Explore agent):** today's `dataentry/watcher.go`
bridge fans out only `{type, id}` entity markers — no audit rows, no payloads.
Same-origin + loopback middleware already wrap the endpoints. The plug is
defensive (regression test + godoc invariant) rather than fixing an active leak.
2. **Mermaid renderer (parallel Explore agent):** already supported
end-to-end via `internal/htmlutil/mermaid.go` (goldmark post-pass)
   + `frontend/src/utils/markdown.ts` (mermaid.js render on mount).
No generator changes needed; just author ```mermaid blocks in the new docs
entities.
3. **docs-project layout:** `docs-project/entities/{concepts,guides}/`,
metamodel declares `concept`/`guide` with optional uncardinalized
`explains`/`covers`/`prerequisite`/`dependsOn` relations.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**

1. **appbuild split.** Port `loadACLPolicy` (returns policy or
non-nil error on malformed yaml; nil on genuine absence) + `buildACL(policy,
store)` (returns the constructed Declarative with a real
`acl.NewStoreGraph(st)`). `prepare()` calls only `loadACLPolicy`; `assemble()`
calls `buildACL` after the store is open. `Collaborators.Declarative` field
added; `WithACL` auto-detects.
2. **`Services.ACLDeclarative()`** accessor exposes the constructed
Declarative. `dataentry.ResolverServices` swaps `ACLPolicy()` for
`ACLDeclarative()`; `ResolverFromProfile` drops the local NullGraph fallback PR
3 left.
3. **`dataentry.attachACLRequest` middleware** — wraps protected
routes; builds one `acl.Request` from the Declarative + the ctx's principal;
attaches via `acl.WithRequest`. Respects already-attached Request (RR-8ZGO).
4. **SSE plug** — godoc on `startStoreEventBridge` documents the
audit-isolation invariant; regression test pumps a denied-write through
entitymanager and asserts no SSE event for it; verify same-origin + loopback
wrapping is intact.
5. **Docs entities** — create `CON-authorization` + `GUIDE-acl-overview`
   + `GUIDE-acl-security` in `docs-project/entities/{concepts,guides}/`.
`GUIDE-acl-overview` carries the mermaid conceptual + sequence diagrams.
Relations via `explains`/`covers`/`prerequisite`.
6. **`appbuildtest.WithDeclarative`** for test wiring (RR-FGJR).

**Files to modify:**

- `internal/appbuild/{appbuild.go,appbuild_fs.go,appbuild_memory.go,appbuild_postgres.go}`
— recipe order, accessor, collaborator field
- `internal/appbuild/appbuildtest/fixture.go` — WithDeclarative option
- `internal/dataentry/{affordances_stub.go,router.go,app.go}` —
ResolverServices interface, middleware
- `internal/dataentry/watcher.go` — godoc + regression test
- `docs-project/entities/concepts/CON-authorization.md` (new)
- `docs-project/entities/guides/GUIDE-acl-overview.md` (new)
- `docs-project/entities/guides/GUIDE-acl-security.md` (new)
- `docs-project/relations/*.md` (new)

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**

- **acl.yaml** — fail boot on malformed (RR-72OJ) so a typo cannot
silently downgrade to NopACL allow-all. Same posture as the metamodel loader.
- **SSE event stream** — no user-supplied content traverses it
today; the regression test pins this invariant so a future change cannot leak
audit attribution data via the broker.

**Security-Sensitive Operations:**

- `attachACLRequest` middleware constructs the Request from the
ctx's principal — never from any client-supplied header. Trust boundary stays at
the principal-stamping middleware that runs earlier in the chain.
- `Services.ACLDeclarative` may return nil when no policy is wired
(NopACL); middleware handles nil by not attaching a Request.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

Each AC names its test (see ticket body). Edge cases:

- acl.yaml absent → NopACL (test pins this hasn't changed).
- acl.yaml present but `roles: {}` → Declarative with empty role
set, all writes denied (existing test coverage).
- Upstream middleware attached a Request → attachACLRequest
doesn't overwrite (RR-8ZGO).
- SSE bridge with no subscribers → no broker traffic, no event
attempts (existing test).

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**

- **Boot-time hard failure on malformed acl.yaml** (RR-72OJ) changes
posture from "tolerate, warn" to "fail." *Mitigation:* this is the explicit
acceptance criterion; matches metamodel loader posture; operator sees clear
error.
- **Resolver interface swap** in `dataentry` — minor
compile-cascade. *Mitigation:* tree builds; full test pass.

Effort: **l**.

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**

- [x] **docs-project/entities/concepts/CON-authorization.md** (new)
- [x] **docs-project/entities/guides/GUIDE-acl-overview.md** (new, with mermaid)
- [x] **docs-project/entities/guides/GUIDE-acl-security.md** (new, member-of hardening)
- [x] ~~docs/metamodel.md~~ (N/A: no metamodel change)
- [x] ~~docs/cli-reference.md~~ (N/A: no CLI change)
- [x] ~~CLAUDE.md~~ (N/A: write-path rules already reference entitymanager)
- [x] ~~README.md~~ (N/A)

## Design Review

- [x] ~~Run `/design-review` before starting implementation~~ (N/A: reference branch already passed cranky-code-reviewer; this PR ports already-reviewed wiring code)
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** RRs from reference-branch review incorporated:
RR-72OJ (critical), RR-FGJR, RR-36UL, RR-JJYW (middleware part), RR-8ZGO,
RR-7O6Q. Plus this PR's two original investigations (SSE audit, mermaid
renderer).
