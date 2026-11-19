---
id: RR-JFUBTP7
type: review-response
title: Pending write retry still says Waits for no one
finding: A run with a failed pending write and no job still shows the no-roles line beside the retry line.
severity: nit
reason: The engine retries a pending write on its own and the step indeed waits for no person; only a failed job needs a manual retry.
status: wont-fix
---
