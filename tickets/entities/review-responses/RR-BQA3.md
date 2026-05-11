---
id: RR-BQA3
type: review-response
title: Mentions includes self-reference — worth assertion that the rendered link is harmless
finding: TestCollectMentions_SelfReference (mentions_test.go line 112) verifies the scan emits the entity's own ID when the viewer references itself. The doc comment on collectMentions says 'the SPA route handles them as no-op navigation.' That's not asserted anywhere — there's no Vue test, no e2e test for the self-reference case. If a future change to the router or to EntityDetail.vue's route-change watch causes a self-link click to reload (or worse, infinite-loop), nothing in the test suite catches it. Add either (a) a Playwright case where origin === target and the link click stays on the same page without an error, or (b) a Vue test for `navigateToEntity` with the current entity that asserts no loadView re-entry.
severity: nit
resolution: Added 'rewrites a self-reference like any other entity link' to markdown.test.ts. Server-side already had TestCollectMentions_SelfReference; now parity on the frontend.
status: addressed
---
