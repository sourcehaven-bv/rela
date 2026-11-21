---
id: BUG-6WQ1U8S
type: bug
title: View section tables on phones, list band and title link colour
description: On a phone a view's table section scrolls sideways instead of stacking its rows, a list leaves an empty band between its toolbar and the table header, and the list title column renders in the text colour instead of the link colour.
priority: medium
effort: m
why1: Each symptom has a local cause. EntityDetail rendered a display table section as a plain table with only overflow-x auto. The list table kept the pane's top padding plus the RlTable top padding, 36px in all. RlTable sets color inherit on the primary cell, and EntityList gave its title link no colour of its own.
why2: The stacked phone layout was built for lists (RlTable compact compress) and dashboard cards (BUG-ST02A0S), but view sections use their own table markup, so they got neither. The primary column option was refused on view sections, so a section could not drop columns either.
why3: The list toolbar moved into the header slot; the pane padding that used to separate them stayed. The title column became the RlTable primary cell, whose colour rule is meant for plain text.
why4: The phone layout is checked by hand per surface, and the e2e mobile spec covered lists and dashboards but no view section.
why5: Tables are built in three places (RlTable, dashboard card, view section) with no shared phone layout, so a fix to one does not reach the others.
prevention: The e2e mobile spec pins that a view section table stacks instead of scrolling and that a list table starts right under the header. Unit tests pin the stacked cell classes and labels, the title link class, and that the server passes primary to the client.
started: "2026-11-21"
completed: "2026-11-22"
status: done
---

## Description

Three data-entry layout problems, found on the harbourmaster account page and
lists in a dark theme.

- On a phone, a view section with `display: table` (such as Subscription,
Invoices and Workspace) scrolls sideways. Lists stack their rows instead and
respect `primary:` columns; sections do not.
- A list leaves an empty band of about 36 pixels between its toolbar and the
table header.
- The list title column, which links to the entity, renders in the text colour
(near white in a dark theme) instead of the link colour.

## Acceptance Criteria

- [x] A `display: table` section 480 pixels wide or narrower shows each row as a heading plus label and value lines, without sideways scrolling.
- [x] `primary: false` on a table section column drops it from the stacked row; `primary:` on other sections and dashboard cards stays a config error.
- [x] Empty read-only cells are left out of the stacked row; editable cells stay.
- [x] Reorder handles, drop lines, actions and inline editing keep working in the stacked row.
- [x] A list table starts right under the toolbar.
- [x] The list title takes the link colour when its column has `link:`.
- [x] Dashboard table cards wrap long values without spaces.
- [x] The behaviour is documented.
