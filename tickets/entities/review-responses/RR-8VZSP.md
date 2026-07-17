---
id: RR-8VZSP
type: review-response
title: 'Grammar not pinned: operator precedence, associativity, and unary not are unspecified'
finding: 'The spec lists operators (and/or/not, ==, !=/~=, <, <=, >, >=, =~) but does not define their precedence, associativity, or a formal grammar. Without a pinned grammar the parser''s behavior on e.g. `a == b and c == d or e` or `not a == b` is a design decision left to implementation, and worse, it may silently diverge from Go internal/predicate (whose whole point is congruence). internal/predicate inherits Lua precedence via gopher-lua; the JS engine must match it deliberately. Required: document the precedence table (Lua order: or < and < comparison < not(unary) is NOT Lua — in Lua `not` binds tighter than comparison; confirm against predicate/eval.go) and associativity, and add parser tests asserting the exact AST for mixed-operator expressions. This is the contract every consumer binds to.'
severity: significant
resolution: 'Grammar pinned in the ticket spec: Lua 5.1 precedence (or < and < comparison < unary not), confirmed against internal/predicate which delegates precedence to gopher-lua. Comparison is non-associative; and/or left-associative. Three sample expressions pinned with required AST-assertion tests, including `not a == b` => `(not a) == b`. AC1/AC7 require the tests.'
status: addressed
---
