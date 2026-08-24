---
id: PLAN-EAQQOX
type: planning-checklist
title: 'Planning: Clear all doclink findings and promote the rule to a blocking CI gate'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:**

IN: clearing all 64 `doclink` findings, promoting the rule to the blocking gate,
and the failure-message guidance that makes suppression a considered act.

OUT: the other four advisory rules (duplication 120, nil-contract 105,
param-contract 5, restatement 17).

**Acceptance Criteria:**

1. `doclink` reports zero on `develop`.
2. `just comment-lint` includes `doclink` and exits 0.
3. Reintroducing a broken link fails the gate AND prints guidance that leads
with fixing.
4. No behaviour change — every edit is inside a comment.

## Research

- [x] ~~For larger features: run `/research`~~ (N/A: `s`, mechanical)
- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A

**Existing Solutions:**

Re-confirmed no existing tool reports these (`go vet`, `staticcheck`,
`godoclint` all zero on a deliberately broken link), and went further this time:
rather than trust commentlint, both flagged shapes were verified against `go
doc` on a minimal package.

- `[Box.unexportedHelper]` → renders with brackets. Go cannot link unexported
members, so the 22 `Type.unexportedMember` findings are real.
- `[Method]` → renders with brackets; `[Recv.Method]` links. So a bare method
reference is real too.

That check mattered: my first pass at auditing these graded 7 of 8 as false
positives on the grounds that the symbols "exist". They do exist — as methods —
and Go still refuses to link them. `grep` was the wrong oracle; `go doc` is the
right one.

Prior art: `plimsoll`'s grandfathering approach (pin the current count, ratchet
down) was considered and rejected below.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**

Clear to zero, then gate. Findings fall into two remedies:

- **Qualify** (18) — the reference names a real linkable symbol but omits its
receiver. `[ForPrincipal]` → `[Declarative.ForPrincipal]`.
- **Unbracket** (38) — Go cannot link the target at all (unexported member,
cross-package symbol not imported, test-only helper). The prose still names the
collaborator, which was the author's intent; the brackets were an unsupported
claim.

Plus 6 pluralized `[X]s` rephrased so the bracket ends the token, and 2 tool
false positives fixed upstream.

**Alternatives rejected:**

1. *Grandfather the count and ratchet down* (the plimsoll pattern). Rejected:
the growth is propagation of existing breakage, so a frozen ceiling still lets
each broken reference spread up to its cap. Zero is the only count that stops
copying.
2. *Suppress the 64 with directives.* Rejected outright — that is exactly the
reflex the guidance message is written to discourage. Suppressing a finding this
mechanical would make the escape hatch the default path.
3. *Leave advisory and hope.* Rejected on evidence: +6 in two days with nobody
fixing any.

**Files to modify:** 30 source files (comments only), `.commentlint.yml`,
`justfile`, `.github/workflows/ci.yml`.

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:** N/A for the edits — every one is inside a
comment, so there is no runtime surface. The CI change installs a pinned version
tag from an org-owned repo through the module proxy, unchanged in kind from the
existing plimsoll/commentlint pins.

**Security-Sensitive Operations:** None. Several edits touch comments in
security-relevant packages (`internal/acl`, `internal/visibility`,
`internal/jwtauth`), so the diff was checked to confirm no line outside a
comment moved — `go build` and the full test suite both pass unchanged.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:**

1. `commentlint -rules doclink` → "no unresolvable doc links across 10351
comments".
2. `just comment-lint` (now `commented-code,doclink`) → exit 0.
3. **Negative:** revert one fix (`[Set.EnforceUpdate]` → `[Set.Enforce]`), run
the gate, confirm exit 1 and that the guidance prints. Then restore.
4. `golangci-lint run ./internal/...` → 0 issues; `go test` over every touched
package → no failures.

**Edge Cases:**

- A comment explaining markdown syntax (`` `[Title](url)` ``) must NOT be
flagged — this was a real false positive, fixed upstream in v0.3.1 with a test.
- Cross-package references to packages that cannot be imported without a cycle
must stay unflagged (already handled at adoption).

**Negative Tests:** scenario 3 above — the gate must actually block, or the
promotion is cosmetic.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl) — `s `

**Risks:**

1. *A blocking gate with an escape hatch trains people to suppress.* The main
risk, and the reason the guidance message exists: it leads with reading the
finding, presents suppression second, and rejects "to unblock CI" as a reason.
Mitigation is social, so it is written where it will be read — at the moment of
failure.
2. *Unbracketing loses information.* Low: the symbol name stays in the prose,
only the (non-functional) link markup goes.
3. *A future rename re-breaks a link.* That is precisely what the gate now
catches — this is the mitigation, not a residual risk.

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**

- [x] `.commentlint.yml ` — gate list and updated backlog counts
- [x] `CLAUDE.md ` — already documents commentlint; the rule list needs the
gate/advisory split refreshed
- [x] N/A for `docs/ ` and `README.md ` — contributor tooling

## Design Review

- [x] ~~Run `/design-review ` before starting implementation~~ (N/A: `s `,
mechanical comment edits. The one design decision — clear-to-zero vs.
grandfather-and-ratchet — is recorded under Alternatives, with the evidence that
ratcheting does not stop propagation.)
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** N/A
