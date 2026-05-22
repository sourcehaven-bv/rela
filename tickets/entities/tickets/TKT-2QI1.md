---
id: TKT-2QI1
type: ticket
title: 'Predicate language: gopher-lua expression subset for declarative conditions'
kind: enhancement
priority: medium
effort: m
status: done
---

## Goal

Land a reusable predicate-evaluation package in `internal/predicate/` that
parses Lua expression syntax via the `gopher-lua/parse` already in the binary,
walks the resulting AST against a strict allow-list, and evaluates against a
host-registered symbol table.

**No ACL integration in this PR.** The package ships standalone so a follow-up
PR can wire it into `acl.yaml` field- and form-level gates and (later) list /
notification predicates.

## Why this approach

Decision context lives in `.ignored/condition-language-use-cases.md` (use cases
+ language choice) and `.ignored/cel-vs-expr-comparison.md` (CEL vs expr
trade-off).

Lua-expression-reuse won the comparison over CEL/expr/EDN because:

- **No new parser, no new binary size.** The parser is already in the binary
for write-path automation.
- **No new CVE-monitoring surface.** Whatever bug class the gopher-lua parser
carries, we already accepted it the day automation scripts shipped.
- **Familiar syntax.** Contributors who've touched automation scripts already
know it; no second-language tax in `acl.yaml`.
- **Comparable security posture to bespoke** once the allow-list rejects
every non-expression node type at parse-walk time.

## What ships

Package `internal/predicate/` with:

1. **Parser-walk** — `parser.Parse` from gopher-lua, then a walker that rejects
every AST node not in an explicit allow-list (~30 node types: comparisons,
boolean exprs, function calls, attribute access, scalar literals, table literals
with allow-listed keys).
2. **Evaluator** — dispatches allow-listed function calls against a fixed
symbol table. Host registers `has_relation`, `count_relations`, `has_role`,
`is_one_of`, `contains`. Globals: `current_user`, `env`.
3. **Static rule-load lint** — unknown symbols and out-of-list operators fail
at `Compile` time, not per request.
4. **Step budget** — per-evaluation visit counter aborts at e.g. 10k.
5. **Fuzz harness** — `go test -fuzz` target over the parse-walk pipeline.

## What does NOT ship in this PR

- `acl.yaml` integration (separate follow-up).
- Use of the package by `entitymanager`, `dataentry`, search, or notifications.
- Temporal facts (`now()`, `hours(n)`, `days(n)`) — deferred per the design
doc; pure evaluator extension when needed, no grammar change.
- Lambda-shaped target predicates for `has_relation` — the table-literal
named-args form covers ~95% of cases per the survey.

## Out of scope

- SPA-side parsing (rules evaluate server-side; SPA receives booleans).
- Multi-hop relation traversal.
- Regex on property comparisons (validation owns format checks).
