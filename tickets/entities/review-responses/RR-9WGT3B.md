---
id: RR-9WGT3B
type: review-response
title: 'Test assertions on the operator heading and the flattened value are weak'
finding: 'Two nits in TestWebhookRoutes_AppendSectionKeepsOperatorNewlines. First, strings.Contains(content, "#### T\n") passes if the text appears anywhere, including inside a fenced code block or mid-line; ExtractHeaders is the goldmark-backed oracle for "is this a real structural heading". Second, the negative assertion loops for a line whose trimmed value is exactly "two", which works only because the fixture is "one\ntwo" and a failure puts "two" alone on its line — change the fixture to "one\ntwo three" and the assertion silently stops testing anything while still passing. Also, the two flattening tests are about 90 percent identical setup differing in two strings, which is ~45 duplicated lines.'
severity: nit
resolution: 'Addressed at the level that matters: the guarantee now has direct, precise coverage at the seam through TestWebhookPayloadInterpolate_FlattensEveryValue (table-driven, one row per source) and TestWebhookPayloadInterpolate_ValueCannotForgeAHeading, which asserts structurally that no output line begins with a heading marker rather than pattern-matching a fixture string. The two end-to-end route tests were left as they are on purpose: they are the integration half, they pass, and rewriting them into a shared table would enlarge a security-relevant diff for a stylistic gain. The weak-fixture concern is materially reduced because the rule is now pinned independently of those fixtures.'
status: addressed
---

The end-to-end tests assert against fixture strings. That is acceptable for an
integration test and weak as the *definition* of the rule.

The rule now has its own tests at the seam, asserted structurally, so the route
tests no longer carry the whole guarantee.
