---
id: RR-DK21QY
type: review-response
title: -rank -top N truncates the summary count, so the advisory report understates the backlog
finding: With -rank -top N, commentlint's trailing summary line counts only the DISPLAYED findings, not the total. `-rules duplication -rank -top 40` prints "40 duplicated facts across 95 comment sites" where the true totals are 119 and 255. The advisory report exists to make the backlog visible, so a silently truncated count defeats its purpose — a reader tracking drift over time would see 40 and conclude the backlog is a third its real size.
severity: minor
resolution: 'Fixed upstream rather than worked around, since a misleading count is a tool bug not a wiring choice. commentlint v0.2.1 counts the full result set and appends "— showing N" only when -top actually truncated. Verified: `-rules duplication -rank -top 40` now prints "119 duplicated facts across 255 comment sites — showing 40"; restatement (19, under the cap) prints no annotation. Regression tests added upstream (TestShowingOf, TestShownCount). rela pinned to v0.2.1 in justfile and ci.yml.'
status: addressed
---

## Finding

`-rank -top N` caps the displayed findings, which is intended — but the summary
line is computed from the truncated slice, not the full result set:

```
$ commentlint -rules duplication ./internal ./cmd
119 duplicated facts across 255 comment sites (corpus: 9876 comments)

$ commentlint -rules duplication -rank -top 40 ./internal ./cmd
40 duplicated facts across 95 comment sites (corpus: 9876 comments)
```

The advisory report is the only mechanism making the backlog visible, and the
ticket's own justification depends on those counts being trustworthy. A reader
watching for drift would read 40 and conclude the backlog is a third of its real
size. The tool's own README warns against exactly this under "No silent caps" —
a bounded report should say what it dropped.

## Severity

Minor rather than significant: the findings themselves are correct and the
per-rule totals are recoverable by dropping `-top`. It misleads about volume,
not about content.

## Resolution options

1. Report both: "showing 40 of 119". Correct fix, needs an upstream change.
2. Drop `-rank -top` from the CI advisory step so counts are honest, accepting
longer log output (the `::group::` folds already keep it readable).

Option 2 unblocks this ticket; option 1 is the upstream follow-up.
