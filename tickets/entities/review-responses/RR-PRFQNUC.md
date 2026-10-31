---
id: RR-PRFQNUC
type: review-response
title: requestCanceled can silence a request whose client is still connected
finding: The errors.Is(err, context.Canceled) half fires when the error comes from another context (singleflight, errgroup) while r.Context() is live; the handler then returns without writing and net/http sends an empty 200.
severity: significant
resolution: 'Fixed: requestCanceled decides on r.Context() only. TestSearch_ForeignCancelIsAnswered pins that a foreign cancel is answered 5xx; the two existing gate tests now cancel the request context.'
status: addressed
---
