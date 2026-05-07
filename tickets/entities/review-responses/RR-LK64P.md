---
id: RR-LK64P
type: review-response
title: Specify deep-copy vs in-place semantics; match shift_headers (deep-copy)
finding: Plan returns 'new_ast' but Approach also says 'mutate text in place.' Pick one. shift_headers returns a deep-copied result via shiftNodeHeaders/deepCopyNode. Match that precedent so users who keep a reference to the original AST aren't surprised.
severity: minor
resolution: Plan commits to deep-copy semantics matching shift_headers (reuse deepCopyNode). The 'in-place' wording is removed. AC19 asserts the input AST is unchanged after resolve_refs returns.
status: addressed
---

# Finding

The plan says `resolve_refs(ast, replacements) → new_ast` but Approach also says
"mutate `text` in place." Pick one. `shift_headers` returns a deep-copied result
(`shiftNodeHeaders` at `markdown.go:806`, `deepCopyNode`).

In-place mutation that also returns the same table is confusing:

```lua
local ast = rela.md.parse(content)
local copy = rela.md.resolve_refs(ast, refs)
print(rela.md.render(ast))  -- already mutated!
```

# Resolution

Commit to **deep-copy**, matching `shift_headers`. Reuse `deepCopyNode`. Update
Approach text. Add test asserting the input AST is unchanged after
`resolve_refs` returns.
