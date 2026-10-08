---
id: TKT-U32AUB
type: ticket
title: Comments across markdown block boundaries
kind: enhancement
priority: medium
effort: l
started: "2026-10-07"
completed: "2026-10-07"
status: done
---

## Description

Users want to comment on a range that crosses markdown block boundaries: a
heading and its body, several paragraphs, or several list items.

The server already accepts such a selection (`quotefind` maps it to source
offsets, and `textanchor.Resolve` finds it again by exact match). Two things
still break it:

1. **Highlighting.** `frontend/src/utils/commentHighlight.ts` splices ONE
`<mark>` pair into the markdown source before rendering. When the range crosses
a block, the open and close tags land in different blocks; the HTML parser
closes the mark at the first block end and drops the stray close tag. Only the
first block is highlighted.
2. **Re-anchoring after edits.** `textanchor`'s fuzzy phase searches a window
inside ONE paragraph. A cross-block quote is therefore found only by exact
match; any edit inside the range detaches the comment.

Suggested replacements stay single-block on purpose (`crossesBlock`); they are
out of scope.

## Direction

Use the markdown AST (goldmark, already used by `quotefind`) as the structure
for both problems:

- Return per-block highlight segments from the server, so the client emits one
`<mark>` per block.
- Resolve each endpoint of a cross-block anchor separately: match blocks first,
then text within the matched block, with the existing whole-document quote match
as the first tier. Research (see planning) found block-level fingerprint
matching gives the useful part of tree matching (GumTree-style) at a fraction of
the cost; full tree edit distance and learned probabilistic TED are not worth it
for shallow markdown trees.
