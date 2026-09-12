---
id: RR-8KQ2MW
type: review-response
title: 'AppendToSection contract drift: a multi-line block is inserted as one line-indexed element'
finding: 'AppendToSection''s parameter is still named `line` and its doc comment still says "appends line", "followed by line", "line is inserted after the LAST content line". The webhook caller now passes a block spanning several lines, and the function is a shared utility in a different package — exactly the drift that bites the next reader. Worse, the implementation works only by accident: `strings.Join(slices.Insert(lines, insert, line), "\n")` inserts a multi-line block as ONE slice element containing embedded newlines. The output is correct because Join does not care, but `insert` and `end` are computed as LINE INDICES over `lines`, so the block silently occupies one index while consuming several visual lines. Any future change that reasons about line counts after insertion (line-numbered diffs, an insert-before-line-N sibling, a cursor offset) will be off by the block''s internal newline count, and the bug is invisible in the common single-line case. TestAppendToSection_PreservesOtherContent already does exactly that line-count arithmetic.'
severity: significant
resolution: 'Renamed the parameter to `block` throughout and rewrote the doc comment to state that it MAY span several lines, that it is split and inserted as separate lines so line indices stay meaningful, and that constraining a block to one line is the caller''s job because only the caller knows which parts are trusted. The insertion is now structurally honest: `slices.Insert(lines, insert, splitContentLines(block)...)`. Behaviour-identical for every existing single-line caller (full markdown and dataentry suites pass unchanged), and it additionally fixes the stray-blank-line case, since splitContentLines("") returns nil so an empty block now inserts nothing. Pinned by two new tests: TestAppendToSection_MultiLineBlock asserts the number of lines added equals the number of lines in the block, and TestAppendToSection_EmptyBlockAddsNothing asserts an empty block leaves the document byte-identical.'
status: addressed
---

The parameter name and the doc comment described a single line; the caller now
passes a block. That is a contract drift a shared utility cannot carry quietly.

The latent bug is the line-index arithmetic:

```go
// before — block occupies ONE index, however many lines it spans
strings.Join(slices.Insert(lines, insert, line), "\n")

// after — one index per line, so the arithmetic stays true
strings.Join(slices.Insert(lines, insert, splitContentLines(block)...), "\n")
```

Both render the same document today. Only the second stays correct for a caller
that counts lines, which the existing
`TestAppendToSection_PreservesOtherContent` already does.
