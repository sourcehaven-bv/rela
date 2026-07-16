---
id: FEAT-8Z47U
type: feature
title: Shared client-side condition expression engine
summary: 'A small hand-written client-side (TS) boolean-expression engine for the data-entry SPA: Pratt/recursive-descent parser + tree-walk evaluator. Supports and/or/not, comparison operators (==, ~=/!=, <, <=, >, >=, =~), parentheses, string/number/bool/nil literals, entity./form. value references, and a pluggable host-function hook (e.g. has_role, has_relation). Grammar deliberately aligned with the Go internal/predicate surface so a future server-evaluated path is a drop-in rather than a rewrite. Reusable across form field/step conditions (visible_when/required_when), ACL-driven button enable/disable, and view-section visibility.'
description: |-
    Several data-entry features need to evaluate a boolean condition against live client-side state (form values, the current entity, the current user) as the user interacts, before any server round-trip:

    - Multi-step/wizard forms: visible_when / required_when on steps and fields.
    - ACL affordances in the browser: enable/disable buttons based on entity state + user roles.
    - View-section / widget visibility driven by config.

    Today the only comparable engine (internal/predicate) is Go-only: it consumes gopher-lua's Lua parser, builds a typed IR, type-checks, and evaluates with depth/step budgets. It cannot be reused client-side without either vendoring a Lua parser into the SPA or reimplementing the whole compiler in JS — and it solves a heavier problem (authorization over the graph) than client condition-checking needs.

    This feature builds a small, self-contained TypeScript expression engine instead:
    - A hand-written Pratt/recursive-descent parser for a boolean-expression grammar (no statements, no scoping, no ambiguity): and/or/not, comparisons (==, ~= and/or !=, <, <=, >, >=, regex =~), parentheses, and literals (string, number, bool, nil).
    - Value references: entity.<field> / form.<field> (and current_user.<field> for the ACL case), read from a caller-supplied bindings object. Prototype-pollution-safe property lookup (reject __proto__/constructor, mirroring utils/filters.ts).
    - A pluggable host-function registry so consumers can register named predicates (e.g. has_role, has_relation) without changing the grammar.
    - Permissive, filter-style value coercion for comparisons (not Lua's compile-time cross-type errors) — empty/unset reads as empty; invalid regex evaluates false + warns, never throws.
    - A small depth cap and defensive evaluation (admin-authored config, but fail-safe).

    The grammar is intentionally kept congruent with the Go internal/predicate surface (== / ~= / and / or) so that: (a) authors see one condition language across server ACL policy and client affordances, and (b) if a condition ever needs host-function logic only the server can answer, the evaluation can move to a server endpoint reusing the Go predicate engine with no grammar change.

    Explicitly NOT in scope: porting internal/predicate's typed IR / compiler / Lua parser to JS; a second full type-checker; any server-side changes. This is a client utility.
priority: medium
status: proposed
---
