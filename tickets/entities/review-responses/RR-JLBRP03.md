---
id: RR-JLBRP03
type: review-response
title: Target mismatch on a PATCH response is reported as a failed save
finding: assertTarget on the save response throws inside the 412 try, so the user sees an untranslated save error for a write that landed.
severity: minor
reason: 'The server PATCHes the address it was given and returns that row, so this path is an invariant check that should never fire. Failing loudly is intended: it stops the bases from moving to another row. Turning it into a translated toast adds a message for a state with no known trigger.'
status: wont-fix
---
