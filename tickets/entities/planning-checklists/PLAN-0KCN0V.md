---
id: PLAN-0KCN0V
type: planning-checklist
title: 'Planning: Access skeleton: mode decisions and request lifecycle'
started: "2026-10-14"
completed: "2026-10-14"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Problem:** dataentry decides on the ACL mode in five closed type switches
spread over five files, and opens the per-request `acl.Request` in a middleware
(router.go) and again in provisioning (provision.go). TKT-WFS778 moves all ACL
work behind `internal/access`. This PR is the second of six (design:
`.ignored/TKT-WFS778-design.md`, PR sequence item 2).

**Scope:** In:
1. New package `internal/access` with a package doc.
2. `access.Mode`: the five switches become its methods, arms unchanged.
`permitsGatedUIElement` (views_handler.go:649), `authorizeElevatedDocument`
(standalone_document_handler.go:87), `revealIsPrivileged`
(history_handler.go:40), `aclPolicyActive` and `commentWritesPermitted`
(comments_wiring.go:152,163), and the read-only arm of `SelectCommandAuthorizer`
(commands.go:189).
3. `access.Opener`: the request lifecycle. `Open` takes over the body of
`attachACLRequest` (reuse of an attached request with the principal check,
`principal_property` resolution, `ForPrincipal`). `Restamp` takes over the
identity part of `maybeProvision` (re-resolve, matched-verified mark,
`ForPrincipal`); stub creation stays in dataentry. One `acl.Request` per HTTP
request; the `http.Handler` shell, `isAPIPath`, the 500 responses and the read
gate attachment stay in dataentry.
4. `acl.Verb` defined from `acl.Op` (thread th-51a613c6): `acl.Ops()` lists
the write ops, `acl.Verbs()` is read plus `Ops()`, `Verb.Valid` and `Verb.op`
derive from `Ops()`, `Op.Valid` is exported and replaces the closed list in
internal/docs. The access reports use `aclmap.Verbs()`: `acl.Verbs()` minus
rename, because the report helpers answer rename wrongly (design review,
TKT-4APGVF). The CLI `--verb` enums are pinned to `aclmap.Verbs()` by a test.
5. `.go-arch-lint.yml`: add component `access`; `dataentry` may depend on it.

Out (later PRs of TKT-WFS778): moving the `readGate` implementations, the gated
reader, write authorization and affordance verdicts (PRs 3 to 5); the other
`a.acl.(*acl.Declarative)` assertions that fetch the policy instead of deciding
on a mode (router.go:270/1094, watcher.go:727, app.go:977/998, me_handler.go:67,
configure_wiring.go:53); the arch-lint deny (PR 6); the audit bugs BUG-2TPO2F
and BUG-HZ4FHZ.

**Acceptance Criteria:**
1. The golden corpus is byte-identical: `go test ./internal/dataentry -run
TestGoldenParity` passes without `-update`, so status, body and store read
counts are unchanged for Declarative, NopACL, ReadOnlyACL and nil ACL.
2. No type switch or type assertion on `acl.NopACL` or `acl.ReadOnlyACL`
remains in dataentry; `access.ModeOf` is the only classifier. Test: grep guard
in `internal/access` tests, table test of every `Mode` method over every ACL
shape (nil, value and pointer Nop/ReadOnly, typed-nil Declarative, Declarative,
unknown impl) asserting the old arms.
3. `attachACLRequest` and `maybeProvision` no longer call `ForPrincipal` or
`ResolvePrincipal`; the existing middleware, provisioning, principal_property
and asserted-identity tests pass unchanged in behaviour.
4. `acl.Verbs()` is read plus every `acl.Ops()` entry and `aclmap.Verbs()`
is that minus rename; the reports reject rename and their output is unchanged.
Test: `TestVerbs_AreReadPlusOps`, `TestVerb_ValidAndOp`,
`TestVerbs_FollowsOpsExceptRename`, `TestReports_RejectRename`,
`TestACLVerbEnumsMatchReportVerbs`.
5. `just arch-lint`, `just comment-lint`, `just plimsoll` and golangci-lint
pass.

## Research

- [x] ~~For larger features: run `/research` to create a structured research doc~~ (N/A: the design is done in .ignored/TKT-WFS778-design.md and archplan threads th-7f905a03, th-51a613c6)
- [x] ~~Searched for existing libraries that solve this problem~~ (N/A: internal refactor)
- [x] Checked codebase for similar patterns or reusable code
- [x] ~~Looked for reference implementations in other projects~~ (N/A: internal refactor)
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A (design doc above)

**Existing Solutions:** `lua.ReadDeps`/`WriteDeps` for the read/write split of
later PRs; `SelectCommandAuthorizer` already selects once at wiring;
`newProvisionSeam` already hides the policy from the write handlers.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**
- `access.ModeOf(acl.ACL) Mode` classifies once into six unexported modes:
unknown (zero value, any implementation it does not know), unwired (nil
interface), no policy (`NopACL` value, pointer or nil pointer), read-only
(`ReadOnlyACL` likewise), policy (non-nil `*acl.Declarative`), broken policy
(typed-nil `*acl.Declarative`). The Mode godoc holds the arm table. Methods:
`PermitsGatedUIElement(ctx, holder, perm)`, `PermitsElevatedRead(ctx, holder,
perm)`, `RevealIsPrivileged()`, `PolicyConfigured()`, `RefusesWrites()`.
`holder` is a one-method consumer-side interface (`HoldsPermission`) that
dataentry's read gate satisfies. The long rationale comments move with the arms.
- dataentry keeps thin call sites where a function adds dataentry config
(`authorizeElevatedDocument` still checks `AllowACLBypass`;
`permitsGatedUIElement` keeps its name because `lint_test.go` pins its callers).
The comment helpers delegate to `Mode`.
- `access.NewOpener(policy *acl.Declarative, verifiedIdentity bool)` rejects
nil. `Open(ctx, log *slog.Logger) (ctx, *acl.Request, error)` returns
`ErrPrincipalMismatch` or `ErrUnresolvedPrincipal`; dataentry maps them to the
same 500 codes and bodies as today and scopes the logger with path and method.
`Restamp(ctx, sub) (ctx, *acl.Request, error)` re-stamps after a provisioned
create; on error the caller keeps its original ctx.
- Where the Opener is built: `attachACLRequest` builds one per router,
behind the existing non-nil `*acl.Declarative` guard in `responseGates` (a
construction error answers 500 `acl_internal`); the provision seam builds one
per call from the live `a.acl`, so `setTestACL` swaps stay honoured.
- `*acl.Request` still crosses the boundary in this PR; PR 3 moves the gate
and stops that.
- Alternatives rejected: a dataentry-local `mode` helper (arch-lint cannot
deny within one package, and PR 6 needs these decisions outside dataentry);
moving the gate implementations now (touches about 80 test sites for no wire
benefit, and belongs with the reader in PR 3); keeping `Verb` as a separate
closed list with a test (a test can be updated along with the list; deriving
cannot be forgotten).

**Files to modify:** `internal/access/{doc,mode,opener}.go` and tests (new);
`internal/acl/{acl,access}.go`; `internal/aclmap/{mapprincipal,can, whocan}.go`;
`internal/docs/assert_acl.go`; `internal/dataentry/{router,
provision,views_handler,standalone_document_handler,history_handler,
comments_wiring,commands}.go` and the tests that call moved functions;
`internal/archguard/aclmode*_test.go` (new guard); `.go-arch-lint.yml`.

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:** The principal on ctx (stamped by the resolver
chain or a verified JWT) and the `--verb` CLI argument. The verb is checked
against the kong enum and `Verb.Valid` (allowlist derived from `Ops()`). No new
input.

**Security-Sensitive Operations:** Every arm of the five switches is kept as it
was. The zero `Mode` is the unknown mode: it hides gated UI, denies elevation
and audits reveals, but (as before) it does not count as a configured policy, so
the comment routes skip their permission checks and accept writes for it. An
unknown implementation is reachable only from tests (the App constructor accepts
nil, NopACL, ReadOnlyACL and Declarative). `Open` keeps RR-875A (500 instead of
fail-open on an unstamped principal), RR-E703 (reuse of an attached request also
attaches a gate) and SIG-2 (principal mismatch is a 500). Error bodies stay
generic (RR-372L); the raw error goes to the server log only.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:** AC1 golden corpus. AC2 `access` table test plus a source
guard. AC3 existing `acl_middleware_test.go`, `principal_property_test.go`,
`asserted_e2e_test.go`, `provision_e2e_test.go`,
`unmatched_reject_wiring_test.go`, plus `access` unit tests for `Open` (fresh,
reused, mismatch, unstamped) and `Restamp` (one match, no match, ambiguous,
lookup error); the archguard guard `TestNoACLModeSwitches` with planted cases.
AC4 cli, acl and aclmap tests. AC5 the lint commands.

**Edge Cases:** typed-nil `*acl.Declarative`; `&acl.ReadOnlyACL{}` and
`&acl.NopACL{}`; nil `acl.ACL`; empty permission string on both permission arms;
elevated document with empty permission; provision race (unique violation) keeps
re-resolve.

**Negative Tests:** unknown ACL implementation denies UI elements and elevation
and audits reveals; `NewOpener(nil, …)` errors; `Verb("rename2")` and `Verb("")`
invalid; the reports reject `rename`.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**
- An arm changes while moving: mitigated by the table test over every ACL
shape and the golden corpus, which runs all four ACL modes.
- Rename in the reports would be wrong (design review): excluded by name
until TKT-4APGVF, so the report output is unchanged.
- Provisioning drops the client ceiling (pre-existing, BUG-AR6NN9): kept,
named in a comment, fixed separately.
- Log lines move package: messages keep their text so log searches still
match.

Effort: m.

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] ~~Docs-checklist will be created when entering implementation~~ (N/A: refactor ticket; the one doc line is covered by the review checklist)

**Documentation Impact:**
- [x] ~~docs/metamodel.md - New metamodel features~~ (N/A)
- [x] docs/cli-reference.md - checked; it does not list the verbs
- [x] ~~docs/data-entry.md - UI changes~~ (N/A)
- [x] ~~CLAUDE.md - New patterns or conventions~~ (N/A: the access rules land with PR 6)
- [x] ~~README.md - Project-level changes~~ (N/A)
- No doc change: report output is unchanged.

## Design Review

- [x] Run `/design-review` before starting implementation (go-architect review; 12 findings recorded as RRs)
- [x] All critical/significant findings addressed in plan (1 critical and 4 significant, all addressed)

**Design Review Findings:**

- RR-35Q8CW, RR-9HCA6O, RR-D0KJFC, RR-DU4YSV, RR-EK9YLX, RR-FJLXEB, RR-H15J7P,
RR-LI0RAV, RR-Q542SJ, RR-T75LSL, RR-WH3FQG, RR-YBE2ZI. All addressed. The
critical one (rename in access reports) is excluded here and tracked in
TKT-4APGVF. The provision ceiling loss is tracked in BUG-AR6NN9.
