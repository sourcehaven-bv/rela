---
id: RR-6A3A7
type: review-response
title: Resolve-on-unmount with false has the same masking issue
finding: 'If a caller does await ask(); doThing() (without branching on the boolean), unmount silently runs doThing because false is returned. Convention: callers MUST branch on the return value. State this in the composable''s docstring; a JSDoc warning is enough. Reject-on-unmount is an alternative but heavier; sticking with resolve(false) + an enforced convention is acceptable.'
severity: significant
resolution: JSDoc on confirm() warns callers that the returned promise resolves to false on unmount and that callers must branch on the boolean. Convention enforced by docstring and test naming.
status: addressed
---
