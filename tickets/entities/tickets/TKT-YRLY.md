---
id: TKT-YRLY
type: ticket
title: Add month/day picker for yearly rrules in data-entry widget
kind: enhancement
priority: medium
effort: s
status: done
---

## Description

The RruleBuilder widget supports yearly frequency but provides no UI for selecting which month and day the event occurs on. For use cases like birthdays or anniversaries, users need to express "every year on [month] [day]" (BYMONTH + BYMONTHDAY in RRULE terms).

Currently: selecting "Yearly" only generates `FREQ=YEARLY` with no date specificity.
Expected: when yearly is selected, show a month dropdown and day-of-month input so the widget generates e.g. `FREQ=YEARLY;BYMONTH=3;BYMONTHDAY=15` for "every year on March 15th".

The monthly frequency already has BYMONTHDAY support in the backend, so this extends the same pattern to the frontend widget.
