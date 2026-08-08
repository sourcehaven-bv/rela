---
id: RR-4DG4KF
type: review-response
title: Pushdown branch never distinctly exercised for the redaction MARK
finding: ScriptReader.ListEntities has two branches — ACL pushdown (RedactRow) and load-then-Filter — and my plan called out that they must not diverge. But TestScriptReads_ListEntitiesMarksRedacted goes through whichever branch the memstore fixture happens to take, pinning neither, and the pre-existing TestListPushdown_RedactsOnEveryBranch uses a stub redactor that strips properties without setting any marker, so it cannot detect a mark divergence either.
severity: minor
resolution: Added TestPushdownBranch_MarksRedacted (internal/visibility/redacted_test.go), which calls PolicyReader.RedactRow (the seam the pushdown invokes per yielded row) and visibility.Redact (what the Filter branch reaches via PolicyReader.redacted) on identical input and asserts the resulting Redacted slices are equal, plus that the pushdown branch does not leak the hidden value. Both branches share Redact by construction, which is why population was placed there rather than in the callers — the test pins that structural property instead of assuming it.
status: addressed
---

## Finding (from cranky-code-reviewer)

My own plan listed "pushdown path vs. Filter path must produce identical markers
— easy to fix one and miss the other" as an edge case, then I did not actually
pin it.

- `TestScriptReads_ListEntitiesMarksRedacted` exercises whichever branch
the memstore fixture happens to select. It proves the marker works on *a* path,
not on *both*.
- `TestListPushdown_RedactsOnEveryBranch` (pre-existing) uses
`countingRedactor`, which strips `"secret"` but sets no marker — so it is blind
to a mark divergence by construction.

## Resolution

`TestPushdownBranch_MarksRedacted` compares the two seams directly on identical
input:

- `PolicyReader.RedactRow` — what the pushdown calls per yielded row
- `visibility.Redact` — what the Filter branch reaches via
`PolicyReader.redacted`

and asserts the `Redacted` slices are equal, plus no value leak on the pushdown
side.

Both branches funnel through `Redact`, which is *why* the population lives there
rather than in each caller. The test pins that structural property rather than
trusting it.
