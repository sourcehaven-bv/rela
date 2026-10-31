---
id: RR-LN4ICA7
type: review-response
title: lateGatedReader.ResolveHeadersErr asserts at runtime
finding: 'Code review: The type assertion could be a compile-time guarantee if scriptReader returned a type that includes ResolveHeadersErr.'
severity: minor
reason: scriptReader returns lua.EntityReader, the interface the Lua bindings take; widening it for one dataentry caller couples the Lua package to the batch reader. lateGatedReader already forwards Family and WriteTarget the same way. The refusal fails closed and the compile-time var assertions in analyze_validations_cap_test.go pin that every production reader has the method.
status: wont-fix
---
