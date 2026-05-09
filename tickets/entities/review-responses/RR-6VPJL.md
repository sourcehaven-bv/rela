---
id: RR-6VPJL
type: review-response
title: Negative tests use bare require.Error; should use ErrorContains for prefix verification
finding: TestMdResolveRefs_NegativeInput and TestMdEntityRefs negative subtests use require.Error which passes for any error. assert.ErrorContains with the expected error prefix would catch regressions where the prefix is dropped or messages become uninformative.
severity: minor
resolution: Negative tests now use assert.Contains(err.Error(), 'rela.md.resolve_refs:') / 'rela.md.entity_refs:' to verify the canonical prefix. Replaced gopher-lua's CheckTable with explicit type checks in luaMdResolveRefs so non-table args also produce the prefixed error.
status: addressed
---

# Finding

Negative tests like `TestMdResolveRefs_NegativeInput` use:

```go
err := rt.RunString(tc.code)
require.Error(t, err)
```

This passes for any error, including a panic-recovered one or a generic "runtime
error" without our prefix.

# Resolution

Use `assert.ErrorContains(t, err, "rela.md.resolve_refs:")` (and substring of
the specific message where helpful). Catches regressions where the prefix gets
dropped or messages become unhelpful.
