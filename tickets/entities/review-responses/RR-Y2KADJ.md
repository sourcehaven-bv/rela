---
id: RR-Y2KADJ
type: review-response
title: Face order changes the face two fallbacks choose
finding: relationTailOr404 and readableFaceOf take the first readable face, which is now declaration order rather than token order (security review).
severity: nit
reason: 'Intended by design section 3.1: declaration order is the tie-break. Both candidates are readable and redacted for the same principal, so nothing new is disclosed.'
status: wont-fix
---
