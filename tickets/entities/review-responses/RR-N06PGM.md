---
id: RR-N06PGM
type: review-response
title: Per-face relation writes are not probed
finding: 'writableRelations leaves FromFace empty, so scope: content edges from one face are not reported.'
severity: significant
reason: 'Same gap as the deferred design finding RR-A5S7TJ. It only affects relations with scope: content on faced types, and the Atlas hooks use none. docs/webhooks.md and the expectation godoc now list it as not compared, so the gate does not claim it.'
status: deferred
---
