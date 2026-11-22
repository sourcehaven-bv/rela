---
id: BUG-UX4P9T2
type: bug
title: Phone list titles, laptop table, banner placement and phone Overview header
description: On a phone a list cuts its title column to one letter and a kept "Download PDF" link to "Download P…"; at laptop width a list drops to the compact layout; the read-only banner sits between a list's toolbar and its table; and the phone Overview puts its notice above an indented heading.
priority: high
effort: m
why1: A compressed row gave every kept cell its full width and the title only what was left; every column had a fixed 150px width, so a laptop-width list beside the sidebar was narrower than the breakpoint plus the columns; the banner slot rendered below the header band that list pages fill; the dashboard drew its own header inside the page instead of in the header band.
why2: The table sized columns from one fixed width with no minimum, and the compact layout had no floor for the title.
why3: The seventh saas-dev browser test was the first to check portal lists with a PDF link column on a phone and at 1024px.
why4: The e2e mobile spec checked column alignment but not the title width, and no test covered a list at laptop width.
why5: Column sizing lived in two templates (header and rows) with no shared rule for preferred and minimum widths.
prevention: columnSizing.ts holds the preferred and minimum widths for the header and the rows, with unit tests that a five-column list fits a 1024px page. e2e tests pin a readable phone title, the laptop table with headers, and the Overview title in the menu bar; a unit test pins the banner above the header.
started: "2026-11-22"
completed: "2026-11-22"
status: done
---

## Description

Found in the seventh browser test of the saas-dev SaaS setup.

- At 375px the portal Invoices list shows "H" as its title and the
  Subscription list "T.". The PDF link reads "Download P…".
- At 1024px lists use the compact layout: no column headers, and Issued and
  Due are gone.
- In a read-only workspace the banner sits between the list toolbar and the
  table, but at the top on an entity page.
- On the phone Overview the notice sits between the menu bar and the title,
  and the "Overview" heading is indented with a large gap.

## Acceptance Criteria

- [x] On a phone a list title keeps a readable width; kept cells move under it when both do not fit.
- [x] The last kept cell shows its value in full ("Download PDF").
- [x] A list at 1024px beside the sidebar shows the full table with column headers.
- [x] Header and row columns stay aligned when columns shrink.
- [x] The read-only banner is at the top of the content area on every page.
- [x] The phone Overview shows its title in the menu bar, above the notice.
