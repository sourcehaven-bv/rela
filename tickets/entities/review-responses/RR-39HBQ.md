---
id: RR-39HBQ
type: review-response
title: Blockquote can contain block-level children, not just inlines
finding: Plan treats blockquote as inline-bearing (replacing 'content' string with 'inlines'). But blockquotes can contain multiple paragraphs, lists, nested blockquotes, code blocks. The existing extractBlockquoteContent already cut corners by joining paragraphs with \n. Promoting to 'inlines' makes that wrong for non-paragraph content. Either give blockquote a block-level 'children' array or document the limitation as 'paragraphs only'.
severity: significant
resolution: 'Per user direction: go goldmark-faithful. Blockquote becomes block-level container with `children`; goldmarkToLua recurses into the children. List items also gain `children` arrays for multi-block items. Pinned in AC2 and AC3.'
status: addressed
---

# Finding

CommonMark blockquotes can contain block-level children: multiple paragraphs,
lists, nested blockquotes, fenced code blocks. The existing
`extractBlockquoteContent` (`markdown.go:725-737`) already cuts corners — it
iterates only `KindParagraph` children and joins with `\n`. Anything else is
silently dropped.

The plan's "blockquote.content → inlines" perpetuates the limitation but
rebrands it. A blockquote with a list inside parses to a `blockquote` node with
no inlines (the list is dropped) and renders to an empty `> ` line.

# Resolution

Two acceptable paths:

1. **Block-level children** — add `children` (an array of block
nodes) to blockquote and recurse `goldmarkToLua` into the children. Renderer
prefixes every line of every rendered child with `> `. This is correct but
expands the refactor's scope.
2. **Inline-only with explicit limitation** — keep the
blockquote-as-inlines approach but document loudly: "blockquote inline content
is the concatenation of immediate paragraph children's inlines, joined with soft
breaks. Nested lists or other block content is not preserved." Add a test that
demonstrates a list-in-blockquote round-trip is lossy.

Recommend (2) for this PR — it's the smaller change and matches existing
behavior. (1) becomes a follow-up if real users hit it.

Either way, **document the choice in scope-out**.
