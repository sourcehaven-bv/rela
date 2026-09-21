---
id: BUG-YVU8CP
type: bug
title: rela.delete_relation silently deletes the default-face edge and reports success
description: 'Manager.DeleteRelation (internal/entitymanager/manager.go:2022-2024) is hardcoded to DeleteRelationState(ctx, from, "", relType, to) — the zero face, unconditionally. Its sibling''s doc comment at :2029-2032 states the consequence outright: a caller holding a state-tailed edge that drops the face ''does not delete roughly the right edge — it deletes the default face''s, and reports success.'' lua.Mutator (internal/lua/deps.go:120) exposes only DeleteRelation, never DeleteRelationState, so a Lua script has no way to reach the correct behaviour. The failure is silent and returns true. Found during design review of TKT-E9BAAA.'
priority: high
effort: s
status: backlog
---

## Symptom

`rela.delete_relation(from, type, to)` on a `scope: content` relation type
deletes the **default face's** edge regardless of which face the caller meant,
and returns `true`.

There is no error, no warning, and no way to express the intended face from Lua.

## Cause

`Manager.DeleteRelation` (`internal/entitymanager/manager.go:2022-2024`) is
hardcoded to the zero face:

```go
// DeleteRelation removes the DEFAULT-tail edge of the triple.
// [Manager.DeleteRelationState] is the general form. **No automation.**
func (m *Manager) DeleteRelation(ctx context.Context, from, relType, to string) error {
	return m.DeleteRelationState(ctx, from, "", relType, to)
}
```

The consequence is already documented on the sibling method
(`manager.go:2029-2032`), which makes this a known-and-recorded gap rather than
a surprise:

> Separate from [Manager.DeleteRelation] for the reason the store draws the
> same line: the tail is part of a relation's identity, so a caller holding a
> state-tailed edge that drops the face does not delete "roughly the right
> edge" — **it deletes the default face's, and reports success.**

`lua.Mutator` (`internal/lua/deps.go:120`) exposes only `DeleteRelation`, never
`DeleteRelationState`, so a script cannot reach the correct behaviour even
knowing the face.

## Why this is worse than the create gap

`BUG-64MU2Q` and `TKT-E9BAAA` concern *creates* that cannot name a face. A
create that cannot express the face **fails loudly** — `ErrFaceRequired`. This
one succeeds while doing the wrong thing to a different row than the caller
named, and the return value says it worked.

## Reproduction sketch

On a faced type with a `scope: content` relation, with edges on both `draft` and
the default face:

1. `rela.delete_relation("POL-1", "cites", "FEAT-1")` intending the draft edge
2. Observe: the default face's edge is gone, the draft edge remains, the call
returned `true`

## Fix sketch

Mirror the create-side work: widen `lua.Mutator` to expose
`DeleteRelationState`, and give `rela.delete_relation` the same trailing opts
table that `TKT-E9BAAA` adds to `create_relation`, so the face is expressible.

The manager-side validation from `RR-9LM7T7` (`requireRelationFaceFor`) should
cover this path too — the scope/declaration rules are identical.

Consider also whether a face-less `DeleteRelation` on a `scope: content` type
should refuse rather than silently target the default, which would make the gap
loud for every other caller (MCP, CLI) until each is fixed.

## Provenance

Found during `/design-review` of `TKT-E9BAAA`, while resolving that ticket's
"may already work" deferral for the update/delete paths. The companion findings:
`update_entity` does carry a faced id correctly; `delete_entity` deletes the
whole family (`RR-AQGG5O`); this one is the silent-wrong-row case.
