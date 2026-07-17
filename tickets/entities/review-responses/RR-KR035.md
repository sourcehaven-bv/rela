---
id: RR-KR035
type: review-response
title: normalizeValue stringifies arrays/objects, producing accidental literal matches; test gaps on nil/coercion/eval-guard
finding: 'normalizeValue does String(raw) for non-scalars, so form.a == ''1,2'' with a=[1,2] is true and objects become ''[object Object]'' -- baffling coincidental matches. Prefer mapping non-scalars to NIL (never-equal-to-a-literal). Also: unknown string escapes silently keep the backslash (\n is literal) -- document or warn. Test gaps to close: numeric-string edge cases (hex/Infinity/whitespace/empty/1e3), =~ operand direction and =~/!= against nil, and the eval-time FORBIDDEN_KEYS guard in resolveRef which is currently unreachable through the parser (dead defense-in-depth with zero coverage -- add a direct-AST test or note it as belt-and-suspenders).'
severity: minor
resolution: normalizeValue now maps arrays/objects to NIL (never equals a literal) instead of String(raw), eliminating accidental matches like [1,2] == '1,2'. Added tests for non-scalar -> nil, =~ against nil value/pattern, and strict-decimal edge cases. The eval-time FORBIDDEN_KEYS guard in resolveRef is now commented as intentional belt-and-suspenders (unreachable via the parser, kept for a future hand-constructed-AST caller). Unknown string escapes keeping the backslash is left as-is (documented in the tokenizer comment; only \' and \\ are defined escapes).
status: addressed
---
