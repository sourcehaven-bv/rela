---
id: REV-19RU54
type: review-checklist
title: 'Review: Remote MCP: lua_eval/lua_run bypass read ACL; search_entities returns unreadable hits'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`)
- [x] Lint clean (`just lint`)
- [x] Comment lint gate clean (`just comment-lint`)
- [x] Coverage maintained (`just coverage-check`)

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

**Review Responses:** RR-TPA5WI, RR-YYUPZO, RR-ARJZD0, RR-0A8FLW (significant, addressed); RR-YVM1X3, RR-RN73N6, RR-VUL7FB, RR-NGLSIT (minor, addressed); RR-I9SG23, RR-P3XYEO (minor, deferred)

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**
PASS: remote tools/list has no lua_* and calls fail (TestRemoteMCP_NoLuaTools). PASS: search excludes unreadable rows (TestRemoteMCP_SearchExcludesUnreadable), hidden titles (TestRemoteMCP_SearchRedactsHiddenTitle) and hidden-field-only matches (TestRemoteMCP_SearchDropsHiddenFieldMatch). PASS: stdio still lists Lua tools (TestDispatch_ToolInventoryMatches).

## Documentation (enhancements only)

Skip this section for bugs and internal refactors.

- [x] ~~Docs-checklist created and linked via `has-docs`~~ (N/A: bug fix)
- [x] User-facing documentation updated (GUIDE-mcp-server, GUIDE-server-security)
- [x] ~~Docs-checklist marked as done~~ (N/A: bug fix)

**Docs Checklist:** <!-- e.g., DOCS-xxxx -->

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
