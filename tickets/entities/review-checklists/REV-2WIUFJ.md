---
id: REV-2WIUFJ
type: review-checklist
title: 'Review: MCP schema hot-reload'
status: done
---

- [x] Automated checks: `just lint` (0 issues), `just arch-lint`, `just plimsoll`, `just comment-lint`, `just coverage-check`, `just ci` all pass.
- [x] Race detector clean on the touched packages.
- [x] All four build tags compile (default, postgres, memorybackend, sqlite).
- [x] Code review performed (cranky-code-reviewer). Findings recorded as review-response entities and addressed.
- [x] Verification: each new test was mutation-checked — the fix was reverted and the test confirmed to fail — for the method-value binding trap, the non-atomic publish, the reload-after-close resurrection, and the searchCloser double-close.
- [x] ~~Manual UI check~~ (N/A: stdio MCP server, no UI.)
