---
id: RR-OF26C
type: review-response
title: Empty-map fast path must precede regex compile to avoid empty-alternation panic
finding: Plan mentions empty-map no-op in Edge Cases but not in Approach. Empty alternation '()' is a regex error in Go; if the early-return doesn't happen first, calling resolve_refs(ast, {}) crashes. Pin it down in Approach.
severity: minor
resolution: 'Plan Approach step 2 places the empty-map fast path before regex compilation: if the map has no entries, return a deep-copied AST immediately, no regex built. AC16 verifies.'
status: addressed
---

# Finding

The plan mentions "empty map → no-op (no regex compiled; early return)" in Edge
Cases but not in Approach. Empty alternation `()` is a regex error; if the
early-return isn't structurally first, a script calling `resolve_refs(ast, {})`
would crash.

# Resolution

Add to Approach:

> **Empty-map fast path.** If `replacements` has no entries (Lua-side
> `next` returns `nil`), return the input AST unchanged immediately. Do
> not attempt to compile a regex.

Negative test: `resolve_refs(ast, {})` returns the AST unchanged and does not
error.
