---
id: RR-0QNNXI
type: review-response
title: Unmarshal no-mutation test only covers an early failure
finding: The no-mutation case used an input that fails in ParseStateRef's separator check, not a later stage.
severity: nit
resolution: Added inputs failing only in ParseFace (KEEP-2@Draft and KEEP-2@).
status: addressed
---
