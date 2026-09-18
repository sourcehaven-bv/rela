---
id: RR-MDH0B1
type: review-response
title: Nested tables get stacked tab stops and the outer region steals the inner table's label
finding: 'wrapTablesForScroll used querySelectorAll(''table''), a descendant query, so a table nested in a <td> was wrapped too: two role=region tab stops over the same pixels. tableLabel had the same flaw via querySelectorAll(''thead th''), so the OUTER region was named from the INNER table''s headers. The JSDoc claimed ''top-level'' while the code did neither. Reachable because the helper also runs on server-rendered document HTML, where an author''s raw <table> survives DOMPurify.'
severity: critical
resolution: 'Skip a table whose parentElement.closest(''table'') is non-null, and scope tableLabel to the table''s own CAPTION/THEAD children with :scope > tr > th. Verified empirically: nested input now yields 1 wrapper labelled ''Table: OUTER'' with the inner table intact. Regression test added.'
status: addressed
---

# Finding

`wrapTablesForScroll` used `template.content.querySelectorAll('table')` — a
**descendant** query — so a table nested inside a `<td>` matched as well. The
double-wrap guard only checked `table.parentElement`, which for an inner table
is the `<td>`, not `.md-table-scroll`.

Reproduced before the fix:

```
count: 2
labels: ["Table: OUTER, INNER", "Table: INNER"]
```

Two distinct defects from one root cause:

1. **Stacked tab stops.** Two nested `role="region"` regions over the same
pixels. A keyboard user tabs into one, tabs again, and lands in another — the
precise "region tells the user nothing" failure `tableLabel` exists to prevent.
2. **Stolen label.** `tableLabel` used `querySelectorAll('thead th')`, also a
descendant query, so the OUTER table was announced using the INNER table's
headers ("Table: OUTER, INNER"). A confidently wrong announcement is worse than
a generic one.

The JSDoc said it wraps "every **top-level** `<table>`". It did not — the
documentation described the intent while the code did something else.

Markdown cannot express a nested table, but this helper also runs on
**server-rendered goldmark HTML** (`DocumentView`, `DocumentsPanel`), where an
author's raw `<table>`/`<td>` survives DOMPurify's default allowlist. Reachable,
not theoretical.

# Resolution

Two scoped queries replace the two descendant ones:

- `if (table.parentElement?.closest('table')) continue` — skips any table with a
table ancestor. Note `table.closest('table')` does **not** work: `closest`
starts at the element itself, so it always matches. That was caught by
re-running the probe rather than by reading the code.
- `tableLabel` now reads `CAPTION`/`THEAD` from `table.children` and uses
`:scope > tr > th`, so it can only ever see its own table's headers.

Verified after the fix: `count: 1`, `labels: ["Table: OUTER"]`, inner table
present and unwrapped. Blockquote and list-item tables still wrap correctly.

Pinned by `wraps only the outermost table when tables are nested` and `wraps a
table inside a blockquote or list item` in `markdown.test.ts`.
