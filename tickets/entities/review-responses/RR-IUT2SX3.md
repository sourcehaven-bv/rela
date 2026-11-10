---
id: RR-IUT2SX3
type: review-response
title: Handler panic values are logged as written
finding: Both queues logged the recovered panic value and the memory queue put it in the error. A handler that panics with a value built from its payload would write entity content to the log.
severity: minor
resolution: 'Addressed: panicSummary logs the panic type, or the text of a runtime.Error, and never the value.'
status: addressed
---
