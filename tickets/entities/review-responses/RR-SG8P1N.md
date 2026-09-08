---
id: RR-SG8P1N
type: review-response
title: TestWebhookConflict_LoserRefindsAndProceeds never invokes the pipeline — it cannot fail
finding: 'The test named as proof that a create-conflict loser re-finds and proceeds does not drive the webhook pipeline at all, which is why the dead retry loop (RR-HI9QIU) shipped undetected. A test that cannot fail is worse than no test: it converts an unverified claim into a documented guarantee, and both the ticket and docs/webhooks.md cite this behaviour as verified. This is the systemic finding behind the critical one — the retry loop was never actually exercised end to end, so the fact that entitymanager strips the store error before it reaches isWebhookConflict was invisible. Fix: rewrite the test to POST concurrently through app.NewRouter() (as TestWebhookConflict_ConcurrentCreateLosesOnUnique does) and assert the loser receives 200 with the WINNER''s entity_id, plus that exactly one entity exists. It must fail against the current code; verify that it does before considering it fixed. Related gap flagged by the same review: the three-workflow test only exercises hooks where find and create types AGREE, so the find/create type-mismatch case permitted by CreateType() (webhook.go:137-148) has no test at any layer.'
severity: critical
resolution: 'TestWebhookConflict_LoserRefindsAndProceeds rewritten to drive the production router: 8 concurrent POSTs to /hooks/alert for the same key, asserting every delivery returns 200, exactly one entity exists, and every response reports that entity''s id. Confirmed it FAILS against the pre-fix code (7/8 losers got 500) before treating the fix as done — the point of the finding was that the old test could not fail. Writing it first is also what surfaced a second bug the review had not found: the test''s own hook config relied on the default {{body.<prop>}} mapping for match:, but the payload field was named differently, so findTarget matched nothing and created a duplicate per delivery. Fixed in the test with an explicit Values mapping and a comment naming the trap.'
status: addressed
---

This is the systemic finding behind [[RR-HI9QIU]]: the retry path was never
exercised end to end, so the error-plumbing bug was invisible to CI.

A test that cannot fail is worse than no test — it turns an unverified claim
into a documented guarantee. Both the ticket and `docs/webhooks.md` cite this
behaviour as verified.

**Fix**: rewrite to POST concurrently through `app.NewRouter()`, as
`TestWebhookConflict_ConcurrentCreateLosesOnUnique` does, and assert the loser
gets `200` carrying the **winner's** `entity_id`, with exactly one entity
stored. Confirm it FAILS against current code before treating it as fixed.

Related coverage gap from the same review: `TestWebhookRoutes_ThreeWorkflows`
only covers hooks where the find and create types agree, so the mismatch case
`CreateType()` explicitly permits has no test at any layer.
