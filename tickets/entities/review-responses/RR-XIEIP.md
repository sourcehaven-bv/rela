---
id: RR-XIEIP
type: review-response
title: formatEntityLabel uses defensive String() coercion not justified by type
finding: formatEntityLabel uses String(entity.properties.title ?? '').trim(). The type of entity.properties.title is string, but the code defends against undefined, null, and non-strings (the test case explicitly casts undefined as unknown as string). If properties really can be non-string at runtime, that's a typing lie elsewhere — and String({foo:1}) would render '[object Object]' in the picker. The defensive coercion either codifies a hidden contract violation without a comment, or is dead code.
severity: significant
resolution: Replaced String() coercion with a typeof-string guard. Entity.properties is Record<string, unknown> in the type system, so the guard is justified — but the new form correctly avoids '[object Object]' or 'undefined' rendering for any non-string value, returning the id alone instead. New test 'selected chip shows id alone when title is non-string (does not stringify object)' locks down the behaviour.
status: addressed
---
