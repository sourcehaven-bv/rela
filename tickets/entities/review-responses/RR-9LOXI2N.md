---
id: RR-9LOXI2N
type: review-response
title: Queue drop past the retry window never reaches the handler
finding: The dispatcher drops a job past its deadline without calling the handler, so its key stays in state.KV.
severity: significant
reason: The ciphertext row is swept after 24 hours on postgres and the memory queue loses it on exit; the leftover key opens nothing. A jobs-package drop hook is a larger change, documented in the metamodel guide.
status: deferred
---
