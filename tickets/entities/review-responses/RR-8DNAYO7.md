---
id: RR-8DNAYO7
type: review-response
title: Batch spends rate tokens before it is accepted
finding: checkLogPost took one token per message before checking it; a refused batch kept the tokens and a batch over a type's rate could never succeed.
severity: significant
resolution: PostBatch checks every message first and then charges all tokens in one locked step (limiter.takeAll). A batch with more messages of one type than its rate allows is a 400. Pinned by TestPostBatch_ChargesOnlyWhenAccepted.
status: addressed
---
