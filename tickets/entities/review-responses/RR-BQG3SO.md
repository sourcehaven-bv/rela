---
id: RR-BQG3SO
type: review-response
title: A reply test that never reaches the reply path
finding: 'internal/realtime/session_test.go:605 "a reply is checked like a post" is refused by the pre-check (service.go:389) before ReplyTo is looked at, so it proves nothing a non-reply case does not. The store-side Admit for a reply is also untested: filechannels runs replyParent before DecideSession (ops.go:49 vs 56), so on that backend an invalid parent wins over an in: refusal. Either drop the case or put a reply in the beforeAppend store-refusal test (TestPost_MessageInStoreRefuses), which is where reply ordering could matter.'
severity: nit
resolution: Dropped the reply case from TestPost_MessageIn.
status: addressed
---
