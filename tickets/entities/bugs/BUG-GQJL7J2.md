---
id: BUG-GQJL7J2
type: bug
title: Flow failure cause, notices and page follow-ups, and a flow runs panel gate
description: A failed job leaves no entry on the run timeline, the next-action banner repeats the step form it links to, a read-only workspace keeps an empty sidebar group, notices lag an action, phone list columns shift per row, an entity page has no title in the phone menu bar, a display checkbox has no name, and the flow runs panel cannot be hidden from a role or a space.
priority: medium
effort: m
why1: A job that failed for good stored its error on the pending job only and wrote no timeline step; the banner did not compare its navigate target with the page; the read-only sidebar dropped the "+" but kept the group; notices refreshed on a 1 second debounce after the change event; a compressed phone cell sized to its contents; the phone menu bar carried only the toggle; a display widget got no label; the panel had no visibility setting.
why2: Each surface was built for its main case (a moving run, a writable workspace, a desktop table, an editable checkbox, a staff-only project) and the other cases were not checked.
why3: The saas-dev config pass with a customer portal was the first to use read-only workspaces, portal roles and phone layouts together.
why4: The timeline, sidebar and next-action tests cover the main states only, and the e2e mobile spec did not compare cell positions between rows.
why5: There is no shared rule that a built-in entity page panel can be gated like the header buttons, and no review step that walks a config as a customer on a phone.
prevention: Tests pin each case (the failure step in the run log, leadsHere, the empty-group filter, refreshNow, the checkbox name, the phone columns e2e, the menu bar title e2e, and entity_panels on the server). entity_panels follows the header_buttons toggle shape so later panels can join it.
started: "2026-11-22"
completed: "2026-11-22"
status: done
---

## Description

Found in the saas-dev config pass with a customer portal.

- A run whose job failed for good shows nothing on its timeline about the
failure. Managers see the stored error; runs that failed before job errors
were stored keep no error text.
- On the `?run=` page the next-action banner offers "Open step", which opens
the page already on screen.
- In a read-only workspace an `items_from` group with no items keeps its
heading with nothing under it.
- After an action, a notice the action settled stays for about a second.
- On a phone a list's kept columns move between rows when a cell is empty.
- On a phone an entity page has no title beside the menu toggle.
- A `render: display` checkbox has no accessible name.
- The Flows panel on an entity page cannot be hidden from portal customers.

## Acceptance Criteria

- [x] A job that fails for good adds a red "failed" entry at its state to the run timeline.
- [x] The next-action banner is hidden when its navigate offer leads to the page on screen.
- [x] A read-only workspace leaves out an empty generated sidebar group.
- [x] An action on the entity page reads the notices again with the view.
- [x] A kept phone column has the same width in every row.
- [x] An entity page shows its type in the phone menu bar.
- [x] A display checkbox is named by its field label.
- [x] `entity_panels.flow_runs` hides the Flows panel per role (permission) or per space, and the server withholds its data.
- [x] The behaviour is documented.
