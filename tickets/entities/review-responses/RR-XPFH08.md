---
id: RR-XPFH08
type: review-response
title: open() does not refuse a row whose link no longer resolves
finding: open() takes any row; if the page state changed between render and click, a row with no link could open and create unlinked. The unit test 'links nothing for a type the page has no link for' was removed.
severity: minor
reason: 'rows and open() read the same reactive state: Vue re-renders the menu before the next user event, so a click always hits a row computed from the current page. Adding a guard in open() would be dead code with no way to reach it through the UI, and the unit test was replaced by row tests that pin which rows are offered.'
status: wont-fix
---
