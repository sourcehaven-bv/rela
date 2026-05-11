---
id: RR-GKET
type: review-response
title: 'e2e: spec creates two features but only the body content path verifies the rewrite — no negative test'
finding: 'e2e/tests/entity-refs.spec.ts has two test cases, both for the happy path: the resolved link renders and clicks navigate. There is no negative case: a code span containing an unknown ID, or a `\`TKT-NOPE\`` mixed with the known one in the SAME content, isn''t verified to remain a <code>. Without that, an over-eager refactor that resolves every code span to /entity/<unknown>/<unknown> would pass the e2e. Add: (a) a spec asserting that a code span with an unknown ID does NOT render as a link; (b) a spec asserting that a code span inside a fenced code block in the body does NOT render as a link. Both are already covered in markdown.test.ts — e2e brings the integration confidence that nothing in the SPA wiring re-introduces them.'
severity: minor
resolution: 'e2e spec extended: ''unknown-ID code spans remain as <code> (no link)'' and ''IDs inside fenced code blocks are not linkified''. Both use new contentCodeSpan page-object helper.'
status: addressed
---
