---
id: RR-673T9IH
type: review-response
title: pingSeq decodes the token through a full Subscribe
finding: pingSeq registers a subscription on every poll to read the token position.
severity: nit
reason: Correct and cheap at this scale; changefeed has no exported decode and adding one for tests is not worth it.
status: wont-fix
---
