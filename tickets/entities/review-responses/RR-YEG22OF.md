---
id: RR-YEG22OF
type: review-response
title: Flows scanned per create
finding: createStarts rescans the flows on every create.
severity: nit
reason: The scan is over the declared flows only, a handful, and runs once per watched create.
status: wont-fix
---
