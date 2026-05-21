---
id: RR-8GOP
type: review-response
title: Prepend `return` trick interacts with source containing `return`
finding: 'If user-supplied source is already `return false` (valid Lua chunk), prepending `return ` yields `return return false`, which is a parse error but a confusing one (''syntax error near return''). Plan must either (a) reject source whose first non-whitespace/non-comment token is `return` with a named error, or (b) document the lexer seam with a test in testdata/reject/. Pick (a); it''s clearer. Add to reject corpus: leading_return.lua.'
severity: significant
resolution: Preprocessor (preprocess.go) rejects source whose first non-whitespace/non-comment token (after BOM strip) is `return`, with named error 'source must be an expression, not a statement'. AC11 + leading_return.lua in reject corpus pin this.
status: addressed
---
