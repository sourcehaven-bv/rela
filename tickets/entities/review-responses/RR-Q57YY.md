---
id: RR-Q57YY
type: review-response
title: Pipe in table cell mangles row on round-trip
finding: 'Cranky review caught: a pipe character inside a code span inside a table cell (real example: REV-1HIEG.md `grep -rn ''htmx|hx-'' ...`) is emitted unescaped, splitting the row into more cells than the header has on re-parse. Verified with reproducing test.'
severity: critical
resolution: Added escapeTableCell helper. Cells now escape `|` to `\|` and collapse `\n`/`\r` to space before being written into table-row slots. Added regression test `synthetic-10` (pipe inside code span inside cell) to TestMdCorpusRoundTrip. Real corpus test (798 in-tree ticket files) now passes.
status: addressed
---
