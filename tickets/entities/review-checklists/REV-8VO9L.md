---
id: REV-8VO9L
type: review-checklist
title: 'Review: Relation-based validation gates are silently dropped; port workflow gates to Lua + enforce done-before-PR'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] ~~All tests pass (`just test`)~~ (N/A: no Go/frontend code changed — change
is a Lua validator + metamodel rules + CI/command config. `go build ./cmd/rela`
confirmed the binary still compiles.)
- [x] Lint clean — YAML validated (yaml.safe_load); Lua header/structure matches
the existing validate-justification.lua pattern
- [x] ~~Coverage maintained~~ (N/A: no Go code; frontend has no coverage gate)

## Code Review

- [x] Run `/code-review` — self-reviewed (config/data change; cranky pass done
inline)
- [x] All critical review-responses addressed (none)
- [x] All significant review-responses addressed (none)
- [x] Self-reviewed the diff for unrelated changes — 4 non-data files, all on-topic

**Review Responses:** none created. Self-review found and fixed one robustness
issue: the CI done-check now reads `status:`/`id:` only within the frontmatter
fence (via an awk `fm_field` helper) so a `status:` mention in a ticket body
can't be misread. Verified against a body-trap fixture.

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist (IMPL-42CX5)

**Acceptance Status:**
- AC1 gates fire: **PASS** — before backfill, the 4 checklist gates flagged
31/27/83/7; a backfilled ticket passes.
- AC2 /pr done-gate: **PASS** — Step 0 added to .claude/commands/pr.md.
- AC3 CI done-check: **PASS** — chore/* now validated; a touched `ready` ticket
fails; frontmatter parser verified on quoted/unquoted/body-trap inputs.
- AC "no regression": **PASS** — `go build` OK; property/`then`-based rules
unaffected; response max:0 gates stay at 0.

## Documentation (enhancements only)

- [x] ~~Docs-checklist~~ (N/A: internal tooling/process refactor, kind=refactor)
- [x] ~~User-facing documentation~~ (N/A: mechanism documented inline in
metamodel.yaml + the Lua header)
- [x] ~~Docs-checklist done~~ (N/A)

## Final Checks

- [x] Commit message explains the why, not just what (3 commits: mechanism,
data prune/backfill, enforcement)
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [ ] Run `/pr` command to create PR and monitor CI
- [ ] All CI checks pass
- [ ] PR URL documented below

**PR:** <!-- to be filled by /pr -->
