---
id: TKT-KRAV
type: ticket
title: Add rrule property type with data-entry UI widget
kind: enhancement
priority: medium
effort: m
status: done
---

## Description\n\nAdd a new `rrule` property type to the metamodel that stores iCal RRULE strings (RFC 5545). The data-entry web UI should render a dedicated widget for building recurrence rules visually instead of requiring users to type raw RRULE syntax.\n\nThe widget should support:\n- Frequency selection (daily, weekly, monthly, yearly)\n- Interval (every N periods)\n- Day-of-week selection (for weekly rules)\n- Day-of-month / last day (for monthly rules)\n- Nth weekday of month (e.g., 1st Saturday)\n- DTSTART (required when INTERVAL > 1)\n- Human-readable preview of the rule\n\nThe stored value is a standard RRULE string that works with `rela.rrule_next()` from the Lua runtime."
