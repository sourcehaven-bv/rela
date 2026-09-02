---
id: RR-YB7XX8
type: review-response
title: 'The legacy type: accommodation suppressed 37 TRUE positives and pinned the wrong conclusion'
finding: Those files load with an EMPTY relation type - the check reported them clean and two table cases asserted that was correct
severity: critical
resolution: 'Confirmed and fixed three ways. The check now reports legacy files under ReasonLegacyTypeKey; the two table cases asserting the opposite were replaced; and the 37 files were REPAIRED (type: -> relation:) after verifying all 37 agreed with their filenames so nothing was lost. One more in docs-project. BUG-2OXEW0''s three blank-typed relations now render as has-review-response. Both projects report clean.'
status: addressed
---

I dismissed 37 findings on the repo's own `tickets/` project as false positives,
"fixed" the check to suppress them, and pinned that conclusion with two table
cases. **The v1 output was right.**

The comment I wrote claimed:

> Those files work — the store keys on the FILENAME, so the content key never
> matters for indexing

The first clause is false and the second is a non-sequitur. Indexing is not the
only thing that reads a relation file: `mdCodec`
(`internal/store/fsstore/markdown.go:335`) builds the relation from **content**
via `doc.getString("relation")`, which returns `""` for a legacy file. And
`type` is not a reserved relation key, so it is additionally stored as a user
property.

Verified on the real project — three relations on `BUG-2OXEW0` rendered with a
**blank type**:

```
→ BUG-2OXEW0 has-review REV-2LX9ON
→ BUG-2OXEW0  RR-28QDBC        <- empty
→ BUG-2OXEW0  RR-E5Q9C8        <- empty
```

This is worse than the #1004 shape it was hiding: #1004 fails loudly downstream
as a cardinality error, this is silently inconsistent, and any read -> write
round trip (formatter, rename, migration) writes the empty type back and
destroys the last record of it.

Fixed three ways: the check now reports these under their own
`ReasonLegacyTypeKey`; the tests assert that rather than the opposite; and **the
37 files were repaired** (`type:` -> `relation:`, after verifying all 37 agreed
with their filenames so nothing was lost). One more in `docs-project`.
`BUG-2OXEW0`'s relations now render with their real type.
