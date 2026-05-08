---
id: RR-GCDTI
type: review-response
title: Table cells stay flat-text but other inline-bearing nodes preserve links — inconsistent contract
finding: Plan keeps table cells as flat strings while paragraphs/headings/blockquotes get full inline preservation. A link in a paragraph survives the round-trip; the same link in a table cell does not. That's an inconsistent contract for the same inline kind. Either bring cells along (cells become inlines arrays; renderer flattens for width measurement) or document the asymmetry loudly with a code-level comment and AC.
severity: significant
resolution: Table cells become inlines arrays. renderTableNode flattens each cell via renderInlines for width measurement and emission. Consistent contract across all inline-bearing positions. Pinned in AC5 (link round-trips), implicitly in AC13 (corpus property).
status: addressed
---

# Finding

Plan: paragraphs/headings/blockquotes get `inlines` (full structure); table
cells stay as strings.

The implication: a link inside a paragraph round-trips (`[text](url)` in,
`[text](url)` out). The same link inside a table cell parses to plain text,
loses URL, renders as `text`. That's an asymmetric contract for the same inline
content depending on where it appears.

The plan justifies it with "cell rendering is column-padded, width-aware
(`runewidth`)". That's a real cost — but solvable: flatten cells at render time
using the same `flattenInlines` policy the rest of the renderer already calls.

# Resolution

Recommend bringing cells along: each cell is an `inlines` array.
`renderTableNode` flattens each cell via `flattenInlines` *before* measuring
width. That's one extra call per cell at render — neglible versus the
consistency gain.

If we choose to keep cells flat:

- Document in the docs caveat list: "table cells are stored as
flattened strings. Inline structure (links, code spans, raw HTML) inside a cell
is lost on parse."
- Add an AC-level test that pins the lossy behavior so future
contributors don't accidentally fix it.

Pick one in the plan and commit.
