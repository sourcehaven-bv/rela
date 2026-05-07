---
id: RR-50CK0
type: review-response
title: Type-set invariant test does not cover all node types
finding: TestMdResolveRefs_TypeSetInvariant fixture only includes heading, paragraph, code_block. The walker has special cases for blockquote, list, table, raw, thematic_break — none exercised. The whole point of the invariant is to cover ALL node types.
severity: significant
resolution: Rewrote TestMdResolveRefs_TypeSetInvariant to use a kitchen-sink fixture covering heading, paragraph, blockquote, list (plain + task), table, code_block, raw (HTML block), and thematic_break. Added an additional sanity check asserting all expected node types are actually present in the parsed AST so the invariant test cannot pass on a degenerate fixture.
status: addressed
---

# Finding

`TestMdResolveRefs_TypeSetInvariant` (`markdown_test.go:1538-1565`) uses a
fixture with only heading, paragraph, code_block. The walker has special cases
for blockquote, list (plain + task items), table, raw (HTML block),
thematic_break — none of which are covered.

The whole point of the type-set invariant test is to catch the case where the
walker accidentally drops or transforms a node of an unfamiliar type.

# Resolution

Use a kitchen-sink fixture covering all node types. Verify:

1. The multiset of `node.type` values is identical before and after.
2. Each node still has its expected text/content/items/header/rows
structure (didn't accidentally turn a string into a table or vice versa).
