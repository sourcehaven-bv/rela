---
id: RR-BHBWJ
type: review-response
title: expectNoBackButton locator keys on text 'Back' — false negative when labelHint resolves
finding: 'base.page.ts:26 expectNoBackButton = locator(''.scope-nav-btn'', { hasText: ''Back'' }).toHaveCount(0). When labelHint.kind === ''list'' and the list title resolves, BackButton renders ''← <list title>'' with no ''Back'' in the text. A future test navigating with ?from=all_tasks and expecting no back affordance would pass this assertion even with the button visibly present. Fix: locate by role/class alone (.scope-nav-btn[href]) or add a data-testid=''back-button'' to BackButton.vue.'
severity: minor
resolution: Added data-testid="back-button" to BackButton.vue. Updated base.page.ts locator to key off [data-testid="back-button"] rather than text content. expectBackButtonVisible / expectNoBackButton now match every BackButton regardless of label text.
status: addressed
---
