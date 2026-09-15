---
id: RR-R1XKH4
type: review-response
title: Three wrong or misplaced comments in validate.go, including a false claim about what breaks
finding: 'The code reviewer found three defects in one 15-line region of internal/dataentryconfig/validate.go, all introduced by this ticket. (1) The new reservation comment claimed a document named _export ''would be unreachable - every request for it would route to the export handler for a document named "" instead.'' That is false: the dispatch matches the reserved segment only in FINAL position, so all four shapes still resolve. Verified by execution - /_documents/_export renders it, /_documents/_export/_export exports it, and the anchored pair works too. isSafePathSegment also permits a leading underscore. (2) The godoc referenced TestExportSegmentMatchesConfig, which does not exist; the real test is TestExportSegmentPremise. A repo-wide grep returned exactly one hit: the comment itself. (3) The new const was spliced between validateDocuments'' doc comment and its declaration, so go doc attributed the merged blob to the constant and validateDocuments lost its invariant documentation entirely. gofmt and go vet are both clean on all three.'
severity: minor
status: addressed
---

## Resolution

1. Rewrote the rationale to say what is actually true: a document named
`_export` is reachable today, and the reservation is **forward compatibility** —
a name that is simultaneously a route keyword is one dispatch change away from
being shadowed silently, and that failure mode is a document that stops
rendering with no error anywhere. The reservation is still right; the stated
reason was wrong.
2. Corrected the test reference to `TestExportSegmentPremise`, and noted that
the alias (`dataentry.exportSegment = dataentryconfig.ReservedExportSegment`)
makes drift a compile error, which is the stronger guarantee.
3. Moved the const above `validateDocuments`' doc comment so each declaration
owns its own documentation again.

## Note for future work

The reviewer's diagnosis is worth recording: three wrong statements in one small
region suggests that block was written from the plan rather than from the code.
It was — the reservation was designed before the dispatch was written, and the
comment was never re-read against what executes. The `doclink` commentlint gate
does not catch an unbracketed test name, and `go vet` does not catch a comment
that is merely false, so nothing automated would have found any of the three.
