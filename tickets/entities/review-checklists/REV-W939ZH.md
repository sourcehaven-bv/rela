---
id: REV-W939ZH
type: review-checklist
title: 'Review: Request-scoped Lua actions: full request in, arbitrary response out'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`)
- [x] Lint clean (`just lint`)
- [x] Comment lint gate clean (`just comment-lint`)
- [x] Coverage maintained (`just coverage-check`)

Run after rebasing onto current `origin/develop` (clean, no conflicts), not
against the stale base the branch was written on. `just lint` reports
`0 issues`; `just arch-lint` and `just plimsoll` are clean; `just comment-lint`
finds no unresolvable doc links.

`just docs-check` FAILED on first run and is the one finding this checklist
surfaced by itself — see Documentation below.

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

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent)
- [x] All critical review-responses addressed
- [x] All significant review-responses addressed
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** RR-J1L1FS (minor, addressed).

No critical or significant findings. RR-J1L1FS is a broken invariant rather
than a demonstrated disclosure, which is why it is minor: `freezeTable` guards
the table it is handed and does not recurse, so `rela.request.query` was
read-only while `query._all` and each per-key list stayed writable — directly
contradicting the doc comment promising the table is the record of what
arrived. Fixed by freezing every level.

Re-verified failing-first during this pass rather than taken on trust:
reverting the nested freezes fails `TestWithRequest_ReadOnlyIsNotSkinDeep` on
exactly the two nested subcases, and restoring them returns the package to
green. The replacement test is a table over every path (top-level field,
existing query key, new query key, header, the `_all` map, a per-key list),
because the ORIGINAL test asserted only the top-level table and that is
precisely why the bug survived.

Self-review of the diff against `origin/develop` found no unrelated changes.

**Security review of the attack surface**, since both ends of this feature are
attacker-influenced and the ticket flagged all of it as needing a decision:

- *Response body/content type* — allowlisted (`application/json`, `text/plain`,
`text/csv`, `application/xml`, `text/xml`); `text/html` refused. Served with
`nosniff`, a `sandbox; default-src 'none'` CSP and `no-store`. Asserted by
`TestAction_RichResponseRefusesActiveContentType`,
`TestAction_RichBodyIsNotHTMLInterpretable` and
`TestAction_RichResponseIsHardened`. Header splitting via a crafted
content type is pinned by `TestAction_ContentTypeCannotSplitHeaders`.
- *Headers* — allowlist, never pass-through, with whole identity-proxy families
(`Authorization`, `Cookie`, `X-Forwarded-*`, `X-Auth-Request-*`, `X-Remote-*`,
`X-Authentik-*`, `X-Pomerium-*`, and the deployment's own principal header)
refused at CONFIG LOAD however they are spelled — the same floor a declarative
webhook gets, so choosing an action cannot buy a weaker rule.
- *Body cap* — the endpoint had none before; now 1 MiB by default under an
8 MiB config-load ceiling, detected rather than truncated.
- *Not a privilege back door* — `TestAction_CapabilitiesStillGatedWithRequestBlock`
and `TestAction_EntityIDStillResolvesWithRequestBlock` pin that a `request:`
block grants no capability and is not a second route to an entity the caller
may not read (BUG-ZWTDH9's gate).

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**

- *Parsed/raw body reaches Lua; JSON as a table, non-JSON as a string* — PASS.
`TestAction_RequestBodyAndQuery`, `TestAction_MalformedBodyDoesNotFailTheRequest`.
- *Query parameters and a safe subset of headers* — PASS.
`TestAction_RequestBodyAndQuery`, `TestAction_HeadersAreAllowlisted`.
- *Status, body and content type settable from Lua* — PASS.
`action_response_test.go` over every `actionStatusFrom` branch, plus
`TestAction_RichResponseAllowsErrorStatus` for the 4xx/5xx case.
- *The existing SPA shape keeps working unchanged; the rich form is opt-in* —
PASS. `TestAction_RequestAbsentByDefault`,
`TestAction_DefaultBodyCapAppliesWithoutRequestBlock`, and the legacy-path
tests in `actions_request_test.go`.
- *`entity_id` still resolves through `visibility.ScriptReader`* — PASS.
`TestAction_EntityIDStillResolvesWithRequestBlock`.
- *Per-action `capabilities:` preserved* — PASS.
`TestAction_CapabilitiesStillGatedWithRequestBlock`.
- *Body size cap* — PASS. `TestAction_BodyCap` (413, not truncation) and
`validate_action_request_load_test.go` for the ceiling at the real load entry
point.
- *Error mapping is explicit* — PASS.
`TestAction_ScriptErrorBeatsScriptStatus`.
- *Motivating Icinga use case works end to end* — PASS.
`examples/icinga-alert.lua` executed through the handler by
`actions_example_test.go`.

**Deliberately NOT delivered:** the declarative route (a config-declared path
such as `/hooks/icinga` instead of `/api/v1/_action/{id}`). The ticket lists it
under "also worth deciding", not scope, and TKT-1EM4KL already shipped
declarative webhook routes for the common mappings. Recording it here so the
omission is a decision rather than an oversight.

## Documentation (enhancements only)

Skip this section for bugs and internal refactors.

- [x] Docs-checklist created and linked via `has-docs` (DOCS-EFMRQM)
- [x] User-facing documentation updated
- [x] Docs-checklist marked as done

**Docs Checklist:** DOCS-EFMRQM

This section found a real defect. The branch had written its documentation
directly into `docs/data-entry.md` and `docs/lua-scripting.md`, but those files
are GENERATED from `docs-project/` entities — `just docs` deleted all 113 lines
of it. The prose was moved to `GUIDE-data-entry.md` and
`GUIDE-lua-scripting.md`, the generated output regenerated, and `just docs-check`
now passes. Left as it was, the entire user-facing documentation for this
feature would have vanished at the next docs build with nothing failing.

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI

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
