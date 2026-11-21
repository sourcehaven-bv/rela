---
id: BUG-L0GT309
type: bug
title: 'Portal polish: label sources; tab title; currency digits; git status request'
description: View and list labels drift from form labels. The tab title flashes the app name. Currency digits vary. The portal asks for git status without git.
priority: medium
effort: s
why1: View fields and columns without a label showed the raw property name; formatAmount dropped the decimals of a whole amount; the title composable wrote the bare app name as soon as the page part was empty; the SPA asked git status on mount regardless of the backend.
why2: 'Each surface picked its own fallback: forms read the schema label, views and lists did not. The tab title had no notion of a page that is still loading. The server did not tell the SPA whether git sync runs.'
why3: Operators then copied labels into data-entry.yaml per view and list, and the copies drifted from the schema label. The amount formatter was tuned for whole euro amounts on dashboards and reused for invoices.
why4: No shared helper owned the label fallback, so each new surface reimplemented it.
why5: Presentation defaults were decided per surface instead of once, with no test comparing a form and a view of the same property.
prevention: withSchemaLabels is the one column fallback with unit tests; format tests pin equal digits for whole and fractional amounts; title tests pin the hold; a capability flag gates the git request.
started: "2026-11-21"
completed: "2026-11-21"
status: done
---

## Description

Findings from a customer portal walkthrough:

- A form shows a property's schema label ("Status") and the entity view shows
the view's own label ("Account status"). View fields and list columns without a
`label:` showed the raw property name, so operators copied labels into
data-entry.yaml and the copies drifted.
- The browser tab shows the bare app name for about a second before the page
title on every navigation.
- `display: currency` shows "€49" next to "€59.29".
- The portal asks `/api/v1/_git/status` on a backend without git sync, and
shows a console warning for a custom property type.

## Acceptance Criteria

- [ ] A view field or column without `label:` shows the schema label of its
property or relation, then the raw name.
- [ ] The tab keeps the previous title while the next page loads; the app
name shows only when no page title arrives within two seconds.
- [ ] A currency amount always shows the currency's minor unit (€49.00).
- [ ] The SPA skips the git status request when `/_schema` reports no git.
