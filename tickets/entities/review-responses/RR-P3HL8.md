---
id: RR-P3HL8
type: review-response
title: Depth cap bounds parse-nesting only, not eval recursion of flat and/or chains
finding: 'MAX_DEPTH is enforced via enter()/leave() around nesting, but a flat left-associative and/or chain is built by a single parseOr/parseAnd call in a while loop, so this.depth stays at 1 regardless of chain length. The resulting AST has an arbitrarily deep left spine; evalNode recurses per operator at eval time. A ~100k-clause chain throws RangeError: Maximum call stack size exceeded inside eval, caught by the outer handler -> silently returns false, so a legitimately-true expression evaluates false (wrong visibility/required) while burning CPU building the string. The docstring implies recursion is bounded; it isn''t. Empirically reproduced. Fix: bound total node/operator count at parse (count per logical operator + comparison in the while loops), rejecting over-long expressions the same way deep nesting is rejected.'
severity: critical
resolution: 'Replaced the nesting-only enter()/leave() depth counter with a total-node budget (MAX_NODES=500) enforced by a this.node() wrapper called for every AST node the parser emits. This bounds flat and/or chains and nested forms uniformly, so a long chain is rejected at PARSE time (constant-false + ''too complex'' warn) rather than blowing the eval-time stack and silently returning false. Tests: a 5000-clause chain is rejected before eval; a 20-clause chain still evaluates correctly.'
status: addressed
---
