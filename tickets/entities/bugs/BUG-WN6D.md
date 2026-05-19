---
id: BUG-WN6D
type: bug
title: Git-conflict-marker detector matches substring anywhere instead of line-anchored
description: |-
    rela's entity loader treats any file containing the substring seven-`<` markers as having unresolved git conflicts and refuses to load it. The check should be line-anchored: only seven-`<` markers at the start of a line (col 0) is a real conflict marker.

    **Impact:** PLAN-ABRRT.md in this very tickets/ project contains the substring inside backticks (documenting how the detector should work). The detector false-positives on it, which silently *skips* the file from validation. That masks 40 pre-existing data-debt errors (tickets stuck in stale states from already-shipped PRs). Discovered when TKT-GN5LN's PR 1 attempted to fix the inline strings — fix exposed the masked debt.

    **Reproduce:**

    1. Create a markdown file with seven-`<` markers anywhere except start-of-line (e.g. inside a code span or quoted in prose).
    2. Try `rela validate` or open the file via MCP.
    3. Observe: file is skipped with "unresolved git conflicts" error rather than loaded.

    **Fix:** anchor the detection regex to `^<{7}` (line start). Same for seven-`=` markers and seven-`>` markers. Real conflicts always start at column 0; substring matches are noise.

    **Related:** TKT-GN5LN (where the issue surfaced) and the data-debt cleanup ticket (filed alongside).
priority: medium
effort: s
status: ready
---
