---
id: RR-V0OE
type: review-response
title: Per-field AST invariants missing — AttrGetExpr.Key must be StringExpr
finding: 'Plan lists accepted ast.Expr types but does not spell out per-field invariants. AttrGetExpr.Key is itself an Expr, not a string: the parser produces &StringExpr{Value:"status"} for entity.status (sugar) but produces an arbitrary expression for entity[expr] (bracket form). Today''s design would accept entity[has_role("x")] as an attribute access. Walker must enforce AttrGetExpr.Key is *StringExpr (dot-sugar form); bracket-indexing rejects with a named error. Apply same discipline to any other ''this field is an Expr but we only allow a constant subset.'''
severity: critical
resolution: Plan section 'Allow-list specifics' now enforces per-field invariants on each accepted node type. AttrGetExpr.Key must be *StringExpr; bracket attr access and computed table keys explicitly rejected. AC2 reject corpus extended with bracket_attr_access.lua, computed_attr_access.lua, computed_table_key.lua.
status: addressed
---
