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
why1: rela's entity loader treated any substring match of `<<<<<<<` as a conflict, refusing to load the file.
why2: The detector used strings.Contains / bytes.Contains rather than a line-anchored predicate.
why3: When the detector was originally written, no quoted-marker case existed in the test corpus, so substring matching was good enough and the line-anchor distinction wasn't surfaced.
why4: The corpus didn't include tickets-about-the-detector because rela's design-doc-as-tickets workflow hadn't been established yet — early tickets were short prose, not multi-page planning checklists with quoted code samples.
why5: rela has parallel implementations of the same predicate (`internal/markdown/parser.go` and `internal/store/fsstore/markdown.go`) and no shared test corpus enforcing the semantics across both. Duplicated logic with no shared contract drifts silently — the bug pattern would have been visible if both implementations were exercised against the same edge-case suite.
prevention: |-
    Pin the line-anchor semantic with the regression tests in
    `internal/markdown/parser_test.go` (`TestParseDocument_ConflictMarkerInCodespan_NotAConflict`,
    `TestHasConflictMarkers_LineAnchored`) and
    `internal/store/fsstore/conflict_detection_test.go`
    (`TestParseDocument_ConflictMarker_LineAnchored`,
    `TestHasLineAnchoredConflict`). Future refactors that drop the
    anchor predicate fail these tests.

    The deeper systemic preventive: the two parallel implementations
    (`internal/markdown/parser.go` and `internal/store/fsstore/markdown.go`)
    duplicate the same predicate. Deduplicating is tracked separately
    so this fix stays small.
status: done
---
