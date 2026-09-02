---
id: RR-PB8DO8
type: review-response
title: Global RouterLink stub's query encoding diverges from vue-router
finding: The global RouterLink stub in test/setup.ts serializes queries with URLSearchParams, which percent-encodes per application/x-www-form-urlencoded, so scope=list:tickets becomes scope=list%3Atickets. vue-router's own stringifier leaves ':' unencoded. The new unit test therefore asserted the STUB's encoding rather than the app's; the e2e test already hedged with a (:|%3A) regex, which was the tell.
severity: significant
resolution: Documented at the stub that it APPROXIMATES vue-router's serialization and that exact-encoding assertions belong in e2e. Made the unit assertion encoding-agnostic (regex over both spellings) so it pins the fact that the scope is carried rather than one spelling of it.
status: addressed
---

**Finding (S4, significant).** The global RouterLink stub in `test/setup.ts`
serializes queries with `URLSearchParams`, which percent-encodes per
`application/x-www-form-urlencoded` — so `scope=list:tickets` becomes
`scope=list%3Atickets`. vue-router's own stringifier leaves `:` unencoded.

The new test at `EntityList.newtab.test.ts:114` therefore asserts the STUB's
encoding, not the app's. The e2e test already hedges with
`/scope=list(:|%3A)features/` (`open-new-tab.spec.ts:50`) — a tell that the two
layers disagree.

Latent trap: a future test asserting an exact href encodes against the stub,
passes, and diverges from what the real router produces.

**Resolution:** document at the stub that it approximates vue-router's
serialization and that exact-encoding assertions belong in e2e; make the unit
assertion encoding-agnostic so it pins the *fact* that the scope is carried
rather than one spelling of it.

Note the stub change itself was right and is strictly better than what it
replaced (`<a><slot/></a>`, which dropped `to` and made every href assertion
vacuous). It does not weaken unrelated tests: adding an attribute cannot break
an assertion that never looked at it.
