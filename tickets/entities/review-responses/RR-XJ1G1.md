---
id: RR-XJ1G1
type: review-response
title: DocumentPage.editButton brittle when label collides with built-in buttons
finding: 'page.getByRole(''button'', { name: label }) matches anywhere on the page. A configured label of ''Refresh'' or ''Back'' (legal server-side) would pick the wrong button. Scope the locator: page.locator(''.header-right'').getByRole(''button'', { name: label }).'
severity: minor
resolution: DocumentPage.editButton(label) now scopes to .header-right via this.headerRight.getByRole(...). A configured label of `Refresh` or any other built-in button name will resolve to the Edit button correctly.
status: addressed
---
