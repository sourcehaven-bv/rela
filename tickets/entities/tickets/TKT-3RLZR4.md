---
id: TKT-3RLZR4
type: ticket
title: 'rela-docs phase 2 (Tier A): markdown+Lua-island doc language + schema/graph resolvers'
kind: enhancement
priority: medium
effort: l
status: done
---

Phase 2 of FEAT-G4VO53, reframed as a **doc language** (see RES-EK7LSA addendum
2026-07-21).

## What

A markdown-with-Lua-islands doc language + the Tier-A resolver library, plus a
`rela docs build <manual.md>` command that runs the preprocessor and emits
resolved Markdown (pandoc-able to PDF). **No browser** — Tier B (`screenshot{}`)
is phase 3.

## The language

Markdown by default; escape into Lua:
- **Statement island** — fenced ` ```rela ` block; doc-API calls append to an output buffer at that position (PHP `echo ` model); supports loops/conditionals (plain Lua).
- **Echo island** — inline `` `rela <expr>` `` span; substitutes the string value mid-sentence.

Preprocessor runs before any markdown renderer sees the file. Raw source renders
as a fenced code block on GitHub (chosen for that reason).

## Doc runtime & the two-graph model

The island runtime binds rela's existing **`ReadDeps `** (read the
documented/real project, read-only) and, for the seed, the existing **`WriteDeps
`** `create `/`update `/`link ` bindings **bound to a throwaway `memstore
`** (discarded post-build). This IS the CLAUDE.md ReadDeps/WriteDeps split:
read→real project, write→ephemeral scratch. No new `seed.* ` API — reuse the
scheduler/MCP write bindings. Sanctioned exception to "no user Lua on the read
path" (offline operator build; writes land in a throwaway).

## Tier-A resolver surface

| Function | Reads | Emits |
| --- | --- | --- |
| `typeref{type, fields} ` | metamodel | field/relation reference table |
| `values{type, field} ` | metamodel | enum values + per-value meaning (phase-1a `descriptions `) |
| `relations{type} ` | metamodel | flat relation list |
| `graph{from, depth, exclude/only, direction} ` | **`tracer `** | mermaid subgraph (type=schema neighbourhood, id=live graph) |
| `lifecycle{type, field} ` | metamodel | mermaid `stateDiagram-v2 ` (phase-1a `TransitionDef `) or flat-list fallback |
| `entity{id, fields} ` | live graph | one entity's values |
| `count{type} ` | live graph | a number |
| `roles_matrix{type} ` | acl | role×entity capability rows |
| `description() ` | metamodel/acl | top-level deployment prose |
| `h1/h2/h3/md(...) ` | — | structural markdown from Lua |

Reuse: mirror `internal/cli/schema.go `'s metamodel walk; `graph ` over
existing `tracer `; mermaid via phase-1a.5's injection-safe
`mermaidStateDiagram `/`mermaidLabel `/synthetic-ids (from HelpModal path).
Depend only on `metamodel `/`acl `/`tracer `/`store `/`lua ` — no browser,
no new heavy deps.

## Hard-first implementation order (per user: "start with the hard stuff first")

1. **Doc runtime + island preprocessor** (the architectural core, riskiest): tokenize ` ```rela `/`` `rela ` `` islands; a Lua runtime binding ReadDeps (real store) + WriteDeps (memstore) + an output-buffer emit API (`h1 `/`md `/statement-island append) + echo-return semantics; fail-loud with `file:line ` on Lua error / bad ref. Prove statement vs echo end-to-end with `md() `/`h2() ` before any resolver.
2. **`graph{} `** — the hardest resolver (tracer traversal + depth + exclude/only + mermaid render + injection-safe labels; type-vs-id dual grain).
3. **`lifecycle{} `** — mermaid stateDiagram + flat-list fallback (reuse 1a.5).
4. **`roles_matrix{} `** — acl walk.
5. The mechanical resolvers: `typeref ` / `values ` / `relations ` / `entity ` / `count ` / `description `.
6. **`create `/`link ` seed bindings** on the memstore (may land with step 1's runtime since it's the same WriteDeps API pointed at memstore).
7. `rela docs build ` command wiring + `--strict ` knob + an example manual under `prototypes/ ` (the ISMS-style corpus) + docs.

## Open questions to settle during planning (none structural)

1. Function vocabulary — lock the naming set before it ossifies.
2. Empty-resolve strictness — silent `"" ` vs warn vs fail; per-build `--strict ` knob (openvwr fails loud).
3. Live-graph reads (`entity `/`count `) in a manual — may warrant a per-build opt-in (distinct from memstore seed data).
4. Where `rela docs ` command + the preprocessor package live (`internal/docs `? `internal/cli `?) and how the memstore + data-entry render helpers are wired without pulling browser deps.

Prototype (this session) hand-resolved a real ISMS "Risicobeheer" chapter
against `sourcehaven-bv/isms-sourcehaven ` to validate the ergonomics.
