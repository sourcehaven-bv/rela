---
id: TKT-K3W8VD
type: ticket
title: 'CLI corpus commands skip non-bare faces; no way to name a world'
kind: enhancement
priority: medium
effort: m
status: backlog
---

Two related gaps in the CLI on a faced project. Both are silent.

## 1. Corpus-wide maintenance commands see bare faces only

`fmt`, `normalize`, `export` and `graph` sweep the whole store with a bare
`store.EntityQuery{}`. `AllStates` defaults false, so
`storeutil.MatchEntityQuery` drops every non-bare face
(`if !q.AllStates && q.World.IsDefaultWorld() && !p.IsDefault() { return false }`).

Sites: `internal/cli/fmt.go:28`, `normalize.go:21`, `export.go:83,122`,
`graph.go:30`, `schema.go:116`, `rename.go:94`.

Verified on a fixture holding `POL-001@published` and `GUIDE-001@nl`:

    $ rela fmt --check
    4 files need formatting (4 entities, 0 relations)

Neither faced file is counted, and nothing says they were skipped. For `fmt`
and `normalize` that means a maintenance command reports a clean corpus over
files it never loaded — the same shape as TKT-4Y6CMV (validation) and
TKT-7CXQVM (MCP).

These four should almost certainly widen to `AllStates: true`: they are
per-FILE operations, so one row per face is the correct unit, not a
double-count. `rename.go` and `schema.go` need their own answer.

## 2. No `--world` / `--face` flag

There is no way to ask the CLI for a world at all. The rationale is recorded
at `internal/cli/kong.go:78`: ~29 unscoped call sites and `readServices` hands
a raw `store.Store`, so a flag added naively "would parse, print nothing
unusual, and serve the DEFAULT world" — worse than no flag.

Note this is now a divergence from the server. Since the server-side
`app.default_world` landed (worlds integration, PR #1452), a bare HTTP request
resolves through the operator's configured world; the CLI ignores
`app.default_world` entirely. Same project, same operator config, two answers.
That is defensible — the CLI's audience is the operator at a shell, who
arguably wants raw rows — but it should be a stated choice, and today it is an
omission.

Faces are already **addressable** by id (`rela show POL-001@published` works)
but not **enumerable**: `list` has no `--face` or `--all-states`.

## Explicitly NOT in scope: the analyze family

`internal/analysis` is already face-aware and was fixed under TKT-4Y6CMV. Its
content checks set `AllStates: true` (`analysis.go:437,537,559`,
`states.go:70`); the identity checks (duplicates, unique, orphans) keep the
bare query **deliberately**, documented at `analysis.go:718-733` — widening
them would report the same entity once per face. Do not "fix" that.

## Related

TKT-7CXQVM (MCP, same shape), TKT-4Y6CMV (validation, fixed),
TKT-O0A8FO (migration). Four packages now; a lint or a constructor that forces
the `AllStates` choice is worth considering over fixing them one at a time.

Found while QA-sweeping the CLI surfaces for the worlds epic.
