---
id: BUG-DFLTCHAIN
type: bug
title: A world selecting a default-marked pointer by name compiles to an unmatchable chain
severity: high
status: backlog
---

**Two functions map a declared pointer name to a stored coordinate, and only
one of them knows about `default: true`.**

- `internal/worlds.declaredPointers` (`worlds.go:179-188`) maps declared names
  through `entity.ParsePointer` and **never consults `PointerDef.Default`** —
  so `draft` compiles to the coordinate `"draft"`.
- `metamodel.StoredPointer` (`copies.go:282`) maps a default-marked name to
  `""`, and its own godoc says getting this wrong "is not a cosmetic bug".

So the store writes the default face at the zero coordinate while the world
compiler looks for it under its declared name.

## Reproduction

```yaml
page:
  pointers:
    draft:     { default: true }
    published: {}
worlds:
  editorial:
    select: [published, draft]
```

```
compiled chain                        = ["published", "draft"]
metamodel.StoredPointer(page, draft)  = ""
```

No row is ever keyed `"draft"`, so the second chain entry can never match. An
entity holding ONLY its default face is **excluded from a world that
explicitly asked for it by name**.

## Why it matters

- It presents to an operator as **"my editorial world is empty and I don't
  know why"** — with no error, no warning, and a config that reads correctly.
- **Nothing rejects it at load time.** The world compiles cleanly.
- `TestCompile_ChainDedup` currently **pins the wrong behaviour as intended**,
  so the bug has a test defending it.

## Fix direction (not prescriptive)

Route `declaredPointers` through the same default-aware mapping
`StoredPointer` uses, so there is ONE definition of "declared name → stored
coordinate" rather than two that disagree — the same shared-predicate
discipline applied to A1/A2 in TKT-T31NKT and to `DeleteRelation`/
`DeleteRelationState` in TKT-C1XUA8 PR-D. Fix the test that pins the current
behaviour, and consider whether a load-time check should reject a chain entry
that can never match.

Found during TKT-WRLDAPI PR-A review; confirmed empirically against the real
compiler. Out of that PR's scope — it is a pre-existing defect in
`internal/worlds`, not in the API surface being added.
