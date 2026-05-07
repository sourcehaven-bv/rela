---
id: RR-2UUJ1
type: review-response
title: No parse benchmark before refactoring a hot path
finding: Plan defers benchmarking. Parse is called per Lua script invocation in scheduled jobs that can process thousands of entities. Allocating per-inline-node Lua table multiplies allocation count. Spend 30 minutes adding a benchmark before/after to verify no >2x regression.
severity: minor
resolution: AC18 adds BenchmarkMdParse on a kitchen-sink fixture. Post-refactor allocs/op and ns/op must be ≤2x baseline; if exceeded, optimize or document in PR description.
status: addressed
---

# Finding

Plan defers benchmarking ("not premature; benchmark only if a real script
complains"). But `rela.md.parse` is called by scheduled scripts that can iterate
over thousands of entities. The refactor multiplies the allocation count per
parse: instead of one block node + one string, we emit one block node + N inline
tables.

For a typical paragraph (~5 inlines) this is 5x the table allocations. For a
kitchen-sink doc (50 paragraphs × 10 inlines) it could be 500x baseline.
Multiplied across N entities, that adds up.

# Resolution

Add a Go benchmark:

```go
func BenchmarkMdParse(b *testing.B) {
    rt := newMdTestRuntime(b)
    defer rt.Close()
    src := loadFixture("kitchen-sink.md")
    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        rt.RunString(`rela.md.parse(...)`)
    }
}
```

Run pre-refactor and post-refactor; verify ns/op and allocs/op don't exceed ~2x.
If they do, decide whether to optimize (e.g. defer building the inline tree
until first access, or pool tables) or accept and document.

Cheap, ~30 min. Worth doing before merge to avoid the "we'll benchmark later"
trap.
