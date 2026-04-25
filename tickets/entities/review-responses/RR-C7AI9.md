---
id: RR-C7AI9
type: review-response
title: test.describe body bypass; rule is weaker than the request.fetch precedent
finding: The rawRequest rule requires a test()/test.only/skip/fixme ancestor, so a `void api.rawRequest('GET', '/x')` placed directly inside a `test.describe(...)` callback (above any inner `test()`) is silently ignored. The existing `request.fetch` ban on lines 54-58 has no ancestor restriction at all, making the new rule strictly weaker than the established precedent. A developer who refactors a setup call out of beforeEach into the describe body will accidentally bypass the rule.
severity: significant
resolution: Dropped the test()-ancestor restriction; the rule now bans api.rawRequest anywhere in tests/**/*.spec.ts, matching the request.fetch precedent. Hooks (beforeEach/beforeAll/afterEach/afterAll) are no longer exempt — verified that no current spec uses rawRequest, including in hooks. The describe-body bypass is closed because the global ban covers all positions in spec files. tests/fixtures.ts remains exempt via the existing relax block.
status: addressed
---
