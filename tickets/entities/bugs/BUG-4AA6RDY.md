---
id: BUG-4AA6RDY
type: bug
title: Next-action banner and entity links misbehave in a space portal
description: The next-action banner keeps Not now after the step is done; banner and link cells lack link roles; entity links miss the space prefix; customers get a link to a run page they cannot open.
priority: medium
effort: m
why1: Links were bare paths and a router guard added the space only on navigation so the href stayed unprefixed; the inform-only notice always showed Not now; the Flows card linked its title for every reader
why2: RouterLink takes its href from router.resolve which guards do not run on; server table cell links and markdown references bypass the router
why3: Spaces came after the link builders and each surface decides its own links and control labels without a shared rule
why4: No test exercised links inside a space; component tests mount without a space and see bare paths as correct
why5: The space prefix was treated as a navigation concern (one guard) rather than a property of every rendered address
prevention: router.resolve prefixes the current space for every link; unit tests pin the wrapper and table cell and markdown hrefs plus the card link and notice label rules
started: "2026-11-20"
completed: "2026-11-21"
status: done
---

## Description

An end-to-end test of a customer portal built on spaces found four defects
around the next-action banner and entity links:

1. The banner's "Not now" control stays on screen after the step it
suggests is done.
2. "Open step" in the banner and link cells of a list (`display: link`) are
read by assistive technology as generic elements, not as links or buttons.
3. Entity links point to `/entity/...` without the `/s/<space>` prefix. They
work through a router redirect, but the address is wrong and a full page load
flashes a bare title.
4. A customer sees the Flows panel's run card linking to `/flows/runs/RUN-...`,
a page they cannot open, because they cannot manage the run.

## Acceptance Criteria

- [x] A notice with nothing to act on offers "Hide", not "Not now". (A
banner whose step is done already disappears; the one seen was an inform-only
notice.)
- [x] "Open step" and link cells expose the link role and a real href.
- [x] Entity links built in a space carry the `/s/<space>` prefix.
- [x] A reader who cannot manage a run sees its status without a link to the
run page.

## Analysis

Reproduced on a copy of the harbourmaster portal (`/s/portal/...`, customer
PSN-001).

1. The banner in question was an inform-only notice (no action, snooze or
dismiss). Its trigger always read "Not now". It now reads "Hide" and offers
"Hide for now" (sends `seen`) next to mute.
2. Not a defect: "Open step" and link cells render as `<a href>`. The
accessibility tree of the test tool reports the inner label span as generic. No
change.
3. `router.resolve` now prefixes the current space, so every RouterLink href
carries it. Table cell links from the server and markdown entity references now
resolve the same way.
4. The run card links its title to the run page only when the reader manages
the run (`_actions.manage`).
