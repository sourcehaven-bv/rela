---
id: REV-GINPCX
type: review-checklist
title: 'Review: Fix malformed YAML frontmatter in AM-feed-field-redaction'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks
- [x] `rela list --project tickets` exits 0 (previously exit 1) and reports 2390 entities
- [x] `rela create ticket --project tickets` works again — this ticket was created with it, which the bug made impossible
- [x] ~~Go tests~~ (N/A: data-file fix, no Go code touched)

## Code Review
- [x] ~~cranky-code-reviewer~~ (N/A: no code change — quoting two YAML scalars in one ticket entity)
- [x] Self-reviewed the diff: only `AM-feed-field-redaction.md`, quotes added, text byte-identical otherwise

## Acceptance Verification
- [x] Root cause confirmed: unquoted colon-space (`visible:-hidden`, `where: clause`) parsed as a nested mapping
- [x] Both affected lines quoted — line 4 errored first, line 5 would have surfaced next
- [x] `rela show AM-feed-field-redaction` confirms title and description round-trip unchanged
- [x] Full re-scan of `tickets/` with line-initial fence detection finds no remaining parse failures

## Documentation
- [x] Ticket records the false positives (`RR-0EWZQW`, `BUG-1VVXHZ`) so the next person does not "fix" two valid files

## Final Checks
- [x] Commit message explains the why (project-wide parse failure, not cosmetic)
- [x] Ready to merge

## Pull Request
- [x] PR opened against develop.
