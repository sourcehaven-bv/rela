---
id: RR-IBJUF
type: review-response
title: Renderer silently drops unknown node types — splice contract must be string-only
finding: renderNode at internal/lua/markdown.go:868 has no default branch — unknown node types are silently dropped. If resolve_refs ever produces or corrupts a node type, content vanishes at render time with no error. The walker must only mutate the text field of existing block nodes; it must not change the type field, introduce new shapes, or add child nodes the renderer doesn't know.
severity: significant
resolution: 'Plan adds a ''type-preserving'' invariant: walker only mutates the text field of existing nodes and never changes a node''s type, introduces new node shapes, or adds child nodes. AC18 asserts the multiset of node.type values is identical before and after resolve_refs.'
status: addressed
---

# Finding

`renderNode` (`internal/lua/markdown.go:868`) has no `default` branch — it
silently drops node types it doesn't recognize. If our walker ever produces a
new node type or corrupts a node's `type` field, that content disappears at
render time with no error.

Implication: the walker must **only** mutate the `text` field of existing block
nodes. It must NOT change `type`, introduce new node shapes, or add child nodes
the renderer doesn't know about.

# Resolution

Add to Approach as an explicit invariant:

> **Type-preserving rewrites only.** `resolve_refs` mutates `text` fields
> (in a deep copy — see `RR-CLONE`) and returns the same node-shape graph.
> It must not create new node types. The renderer (`markdown.go:868`)
> silently drops unknown node types, so any deviation here would manifest
> as silently-vanishing content.

Add a sanity test that walks the AST after `resolve_refs` and asserts the
multiset of node `type` values is unchanged.
