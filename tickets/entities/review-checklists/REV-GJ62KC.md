---
id: REV-GJ62KC
type: review-checklist
title: 'Review: Runtime under the load line: extract elevation/output/schema-sort clusters (45 → ~37)'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass — verified at the real commit with `-count=1`: ./internal/lua/... plus script, dataentry, automation, autocascade, entitymanager; `-race` on internal/lua
- [x] Linters pass (plimsoll at 37, arch-lint, comment-lint clean across 11093 comments, go vet, golangci-lint 0 issues)
- [x] Coverage floors hold

## Code Review

- [x] `/code-review` run (cranky-code-reviewer, diffed against the STACKED base tkt-dopcti-lua-bindings, not develop)
- [x] All critical/significant findings addressed (none)
- [x] Minor finding recorded: [[RR-DH5FHU]] — the PR incidentally repairs a pre-existing detached-godoc bug

## Verification

- [x] **ACL elevation guarantee intact:** the gate in registerBindings is unchanged (`ElevatedManager != nil || ElevatedReader != nil`, still nested inside `allowWrites`), so an elevationBindings exists only when a handle was wired — unwired capabilities remain STRUCTURALLY ABSENT, not present-and-erroring
- [x] nil-em / nil-er asymmetry byte-identical: write methods absent when unwired, read methods present-and-raising (the TKT-Y3JVFK contract)
- [x] Audit path byte-identical: the defer still flips `live = false` AND calls recordElevatedReads on both exit paths; recordElevatedReads/readUsage/mark/registerElevatedWrites/registerElevatedReads/elevatedGetEntity/elevatedListEntities/elevatedGetRelations all unchanged. 24 elevation tests pass unmodified, including AbsentWithoutAnyHandle, ReaderOnlyHandle, AuditsEvenWhenClosureRaises, DeniedReadIsNotAudited
- [x] `ctxFn: r.callerCtx` is a bound method value on a pointer receiver — re-reads parentCtx at CALL time, not a snapshot. Had it been `ctx: r.callerCtx()`, elevated writes would have lost the caller's Principal and triggered_by (silent attribution hole). Verified correct
- [x] **outputBindings value-capture verified safe:** every write to stdout/outputDir/isAction/isDocument/deps.ProjectRoot was traced — all are Option closures applied before r.out is built, and the only post-construction mutators (SetScriptPath, SetArgs) touch none of them. The godoc explicitly contrasts this with cacheBindings' scriptPath closure so the next reader is told why the two clusters differ
- [x] Method count verified at 37, matching the directive — Runtime is under the 40 load line
- [x] Zero test-file changes; the only non-receiver edit in the whole diff is a local rename (`b` → `bo`) forced by the new receiver name

**Reviewer note on the arc:** each new file carries a "why a type instead of
more Runtime methods" godoc pointing back at the urlHelpers rationale — worth
keeping up, since it saves each successive reviewer from re-deriving the
reasoning.
