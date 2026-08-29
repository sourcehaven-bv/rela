---
id: REV-VF93AM
type: review-checklist
title: 'Review: Extract markdown AST helpers off lua.Runtime (plimsoll ratchet 105 → ~60)'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (full `go test ./...`; `-race` on internal/lua)
- [x] Linters pass (golangci-lint 0 issues; plimsoll with directive ratcheted 105 → 60; arch-lint; comment-lint gates clean)
- [x] Coverage floors hold (no package floor regression — moves within one package)

## Code Review

- [x] `/code-review` run (cranky-code-reviewer, independent verification of the diff against develop)
- [x] All critical/significant findings addressed (none found — verdict: sound, pure structural move; non-mechanical diff is only type declarations + imports)
- [x] Minor findings addressed: [[RR-KZLSV1]] (ls comment accuracy + two leftover receiver shadows), [[RR-XHIH9N]] (ctx closure rationale) — both fixed on the branch

## Verification

- [x] Behavior preservation verified: reviewer confirmed nil-reader/nil-meta error paths byte-identical, deps never mutated post-construction (capture-at-registration safe), ACL gate on entity_refs still pinned by TestScriptReads_EntityRefsGated
- [x] Method counts independently verified: Runtime 60, mdHelpers 26 / mdASTConverter 16 / mdEntityRefs 3

**Notes:** Reviewer's deferred leverage idea (drop mdASTConverter.ls entirely —
NewTable is receiver-independent) recorded on [[RR-XHIH9N]] for a later ratchet
step.
