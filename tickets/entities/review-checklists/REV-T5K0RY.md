---
id: REV-T5K0RY
type: review-checklist
title: 'Review: Command exec ungated under default NopACL: refuse on non-loopback bind, with an explicit override flag'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`go test ./...` green; also `go build` clean under the
default, `postgres` and `memorybackend` tags, preserving backend-import
separation)
- [x] Lint clean (`golangci-lint run ./...` — 0 issues; `gofmt -l` clean;
`just arch-lint` OK; `just plimsoll` passes; markdownlint 0 issues over
`docs/server-security.md` + `docs-project/**`)
- [x] Comment lint gate clean (`just comment-lint` — no unresolvable doc links
across 15335 comments; this is what confirms the `SelectCommandAuthorizer`
capitalization fix in RR-0ECB39 actually resolves). `just comment-report` shows
two advisory findings, both pre-existing and outside this diff's hunks
(`main.go:332 buildIdentityVerifier`, `projectsetup/app.go:30`) — none
introduced here.
- [x] Coverage maintained (`just coverage-check` PASS: package floor 50% and
total 65% both satisfied; total 79.7%)

**Comment findings.** `just comment-report` lists the advisory rules
(duplication, nil-contract, param-contract, restatement). They are not a merge
gate, but a finding your diff *introduces* should be fixed or suppressed — don't
grow the backlog.

Every rule is a heuristic over prose, so false positives are expected. To
suppress one, prefer the inline form on the declaration line, which travels with
the code and is reviewed in this diff:

```go
func f(p string) {} //commentlint:ignore param-contract  p is contained by Clone
```

Use `.commentlint.yml` (`ignore:` path globs, `allow-phrases:`) only when the
same prose recurs across many sites. A reason is required either way — an
unexplained suppression is a finding nobody can re-evaluate later.

## Code Review

- [x] Run `/code-review` command — ran both reviewers in parallel per the skill:
cranky-code-reviewer (general quality) and the project's rela-security-reviewer
(security only, OWASP-framed against rela's own invariants).
- [x] All critical review-responses addressed — **neither reviewer found a
critical.** Both independently tried to construct a fail-open and could not.
The security reviewer additionally traced the supply chain and confirmed
`appbuild.buildACL` returns `(d, d)`, so an `active` of `*acl.Declarative`
implies `decl != nil` and the gated arm can never be skipped into the
loopback/override arm for a policied deployment.
- [x] All significant review-responses addressed — RR-AG09JL (silent refusal)
and RR-SZSMDM (untested `isLoopbackHost`) fixed in code; RR-E979QX deferred to
TKT-PYPNWO with reasoning recorded (see below).
- [x] Self-reviewed the diff for unrelated changes — 36 files: 12 code/docs +
24 ticket entities/relations. Audited after a concurrent-write incident:
`git diff` over `cmd/ internal/ docs/ docs-project/` against the
pre-incident commit was empty, and no `frontend/package*.json` churn survived.

**Review Responses:** RR-AG09JL (significant, addressed), RR-SZSMDM
(significant, addressed), RR-E979QX (significant, **deferred**), RR-I29U51
(minor, addressed), RR-0ECB39 (minor, addressed). Plus the four design-review
responses closed earlier: RR-QWVG8Y, RR-CWBZVT, RR-8HJYDL, RR-2TVXO7.

**On the one deferral (RR-E979QX).** The security reviewer found that
`/api/open-file` spawns an OS opener via `cmd.Start()` on the same mux as
`/api/command/` with no authorizer at all — so on a non-loopback bind with no
`acl.yaml` it stays reachable unauthenticated. It is pre-existing, and bounded
(argv array with `--` flag-stopping, `containedProjectPath` containment,
compile-time-constant program name, no output returned), so it is remote
process spawn plus a file-existence oracle rather than the RCE this ticket
fixed. It is deferred rather than fixed here because **TKT-PYPNWO already owns
exactly this surface** and already proposes refusing `/api/open-file` on a
non-loopback bind; gating it here would widen this ticket past its
command-exec seam and half-fix PYPNWO, leaving its no-op-on-headless half open.

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**

- AC1 loopback + no policy → ungated → 200: **PASS** —
  `TestCommandExecNopACLFailsOpen` (all four contexts).
- AC2 network + no policy + no override → deny → 403 and button omitted:
  **PASS** — `TestSelectCommandAuthorizer/"nop + network, no override → deny"`,
  `TestCommandExecReadOnlyDenied`, `TestResolveCommandsFiltersUnauthorized`.
- AC3 network + no policy + override → ungated, warning fires: **PASS** —
  `TestSelectCommandAuthorizer` asserts `OnOverride` fires on exactly that path.
- AC4 Declarative held / not-held / view: **PASS** —
  `TestCommandExecDeclarativeFailsClosed` (granted 200; not-held 403; no
  permission 403; `view` 403 even when granted).
- AC5 ReadOnly → 403 despite a permissive ctx gate: **PASS** —
  `TestCommandExecReadOnlyDenied`, the RR-CWWJGW canary, preserved.
- AC6 desktop / in-process (no bind) → ungated: **PASS** —
  `UngatedCommandAuthorizer()` wired in `cmd/rela-desktop` and
  `internal/docscapture`; both verified by the security reviewer as having no
  network listener.
- AC7 no `acl.ACL` type-switch in command source: **PASS** —
  `TestCommandAuthorizationHasNoACLTypeSwitch`.
- Added during review: the refusal hook fires on exactly the
  network-no-policy-no-override path (`wantOnRefuse` column), and
  `TestIsLoopbackHost` pins the predicate, with its 10 false cases asserting
  the fail-closed direction deliberately.

## Documentation (enhancements only)

Skip this section for bugs and internal refactors.

- [x] Docs-checklist created and linked via `has-docs` — DOCS-SKMBCW. An
earlier revision of this checklist marked this N/A on the reasoning that the
docs shipped in the same diff; the `done-enhancement-needs-docs-done` validation
rule requires an actual linked docs-checklist entity for a done enhancement, and
CI was right to reject the shortcut.
- [x] User-facing documentation updated — `GUIDE-server-security` gains the
mode/bind decision table, the `--allow-unauthenticated-commands` guidance, and
an upgrade note for deployments that relied on the old ungated default;
`GUIDE-acl-security` gains the matching "depends on the policy AND the bind"
section. Both corrected during review (RR-I29U51): the block was relocated under
its proper heading and the flag example fixed to `--bind 0.0.0.0 --port 8080`.

  Note the *source*: `docs/server-security.md` and `docs/acl-security.md` are
  GENERATED from the `docs-project/` guide entities. The first push hand-edited
  the generated file, and the Docs CI job caught it by regenerating and diffing.
  The prose now lives in the guide entities, with the generated files reproduced
  from them.
- [x] Docs-checklist marked as done — DOCS-SKMBCW is `status: done` with every
item checked or skipped with a reason.

**Docs Checklist:** DOCS-SKMBCW (done).

## Final Checks

- [x] Commit message explains the why, not just what — names the pre-existing
NopACL fail-open, the deployment shape it exposed, and why refusal is the
default with an explicit operator opt-out.
- [x] No TODOs or FIXMEs left unaddressed — none introduced; `git diff` over
the branch shows no new TODO/FIXME markers.
- [x] Ready for another developer to use — the decision matrix is in the
`SelectCommandAuthorizer` godoc, the fail-open trap it avoids (`nopReadGate`
answering permissive under both NopACL and ReadOnly) is documented at the seam
rather than only in a ticket, and two grep guards
(`TestCommandAuthorizationHasNoACLTypeSwitch`,
`TestCommandTestsDriveAuthorizerNotACL`) stop the old shape returning.

## Pull Request

- [x] ~~Run `/pr` command to create PR and monitor CI~~ (N/A at checklist-close
time: `/pr` requires the ticket to be `done` and validating clean BEFORE it
opens the PR, and a `done` review-checklist may have no unchecked items — so
this item cannot be truthfully checked by the checklist that gates it. Marking
it skipped rather than checked avoids asserting a PR exists when it does not;
see the note below, and TKT-UFV01M.)

<!--
Deliberately NOT tracked here: the PR URL and whether CI passed.

Both post-date this checklist. `/pr` requires the ticket to be `done` and
validating clean before it opens the PR, and a `done` review-checklist may have
no unchecked items — so an item asking for the PR URL can only be satisfied by a
PR that does not exist yet. Checking it early would mean asserting "CI passed"
before CI ran, which turns the checklist from evidence into a formality.

GitHub records both authoritatively, and the branch and commit messages carry
the ticket ID, so the ticket-to-PR link is recoverable without duplicating it
here. See TKT-UFV01M. -->
