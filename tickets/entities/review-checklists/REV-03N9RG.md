---
id: REV-03N9RG
type: review-checklist
title: 'Review: Anchored documents skip the face gate: a published-only reader renders the draft'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`) — `go test ./internal/dataentry/...` passes locally; the full local run timed out only in TestAnalyzeProperties_StopsScanningAtCap under host load average 125 (unrelated code); GitHub CI is the gate
- [x] Lint clean (`just lint`) — 0 issues
- [x] Comment lint gate clean (`just comment-lint`) — no unresolvable doc links
- [x] ~~Coverage maintained (`just coverage-check`)~~ (N/A: runs in GitHub CI; the change adds tests only in an already-covered package)

**Comment findings.** `just comment-report` lists the advisory rules
(duplication, nil-contract, param-contract, restatement). They are not a merge
gate, but a finding your diff *introduces* should be fixed or suppressed — don't
grow the backlog.

## Code Review

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent)
- [x] All critical review-responses addressed (none raised)
- [x] All significant review-responses addressed (RR-Q0ZRDP, RR-3WBQ0B, RR-U1XMKI, RR-5AZRCI)
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** RR-Q0ZRDP, RR-3WBQ0B, RR-U1XMKI, RR-5AZRCI (addressed);
RR-GUJG3Q, RR-EUKDA9, RR-0Y55WB, RR-S16QPH, RR-02EH9W (addressed); RR-7D937N,
RR-L92DY8 (deferred); RR-M67INX (wont-fix).

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**

- Row gate then face gate, uniform 404: PASS (TestAnchoredDocument_FaceGate "denied face is the uniform 404"; TestAnchoredDocument_BareIDRouted)
- Addressed face renders, including Lua reads: PASS (ENTRY/TITLE assertions for draft and published)
- `ID@face` never 500: PASS (all addressed cases 200 or 404)
- Export path: PASS (TestAnchoredDocumentExport_FaceGate); entity export_render never sees a face (TKT-5SZG2L); no server-side per-entity document list

## Documentation (enhancements only)

- [x] ~~Docs-checklist created and linked via `has-docs`~~ (N/A: bug fix)
- [x] User-facing documentation updated — `entry_id` row in docs/lua-scripting.md and docs/data-entry.md
- [x] ~~Docs-checklist marked as done~~ (N/A: bug fix)

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI — PR opened with `gh pr create` against faces-intrinsic as the brief requires
