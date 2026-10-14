---
id: RR-Y1KTOI
type: review-response
title: allowAll covers only declared fields
finding: A block-less verb holder adds only the declared universe. A field or option named by another role's block but undeclared would stay denied with that role and visible without it, breaking monotonicity.
severity: minor
reason: Grant validation rejects a block naming an undeclared field at compile time, so the declared universe covers every candidate a block can add. Recorded in the allowAll doc comment.
status: wont-fix
---
