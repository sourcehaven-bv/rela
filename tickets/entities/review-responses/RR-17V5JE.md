---
id: RR-17V5JE
type: review-response
title: Comment claimed ListEntityHeaders carries 280,000 ids; it carries one
finding: The comment justifying the whole test said resolveRelationColumns 'ends in one ListEntityHeaders ... whether it carries 2,000 ids or 280,000'. Measured breadth for this fixture is ListEntityHeaders=1/1 - a single id. relationColumnTargets dedupes target ids via seenTarget (views_handler.go:909-913), so the header batch is bounded by DISTINCT NEIGHBORS, not row count; the fixture assigns every ticket to one person, collapsing the target set to a singleton. The load-bearing sentence in the test's own rationale was false, and the ListEntityHeaders leg is structurally unexercised by this fixture.
severity: significant
resolution: Comment rewritten. It now states that the measured leg is ListRelations, explains that ListEntityHeaders is bounded by distinct neighbors rather than row count because of the dedup, and says explicitly that this fixture's single assignee makes that call carry one id whatever the section does. The header leg is named as out of scope rather than silently implied to be covered.
status: addressed
---
