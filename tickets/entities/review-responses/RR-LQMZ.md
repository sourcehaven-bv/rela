---
id: RR-LQMZ
type: review-response
title: 'e2e: confirm the rendered link is not produced by an unrelated path — currently inferred'
finding: 'e2e/tests/entity-refs.spec.ts seeds an origin feature whose content contains a backticked target ID, then asserts that the entity detail page contains an anchor at `/entity/feature/<targetId>`. The link COULD in principle be rendered by some other path (a derived ''see also'' panel, a related-entities section, a renderer that auto-linkifies known IDs). The test doesn''t pin down that the link was produced by the codespan-rewrite path. Today no such alternative path exists, but the assertion is structurally fragile. Suggest scoping the locator further: assert the anchor is INSIDE the .content-body or specifically inside `.markdown-content` (the v-html target) so any future automation that adds an unrelated link won''t make the test pass for the wrong reason. Even better: query for both the absence of `<code>TARGETID</code>` AND the presence of the link, so a regression that silently leaves the code span untouched AND adds a link via another path is caught.'
severity: nit
resolution: contentEntityRefLink is scoped to this.contentBody so it cannot match sidebar/relation-card links. Negative test also asserts contentCodeSpan(targetId).toHaveCount(0) -- link present + code span absent is airtight.
status: addressed
---
