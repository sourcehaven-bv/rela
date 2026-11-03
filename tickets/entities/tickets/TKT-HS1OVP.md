---
id: TKT-HS1OVP
type: ticket
title: 'Entity header: one Convert menu and overflow for actions that do not fit'
kind: enhancement
priority: medium
effort: m
started: "2026-10-24"
completed: "2026-10-24"
status: done
---

## Description

On an atlas topic with three schema conversions, each conversion was its own
header button. Labels wrapped to three lines and the title shrank to one letter
per line.

## Change

- All allowed conversions share one **Convert** menu; one conversion stays a
button.
- When the row does not fit beside the title, the commands, the header create
menu and Edit stay; the rest moves into the "⋯" menu built from the same action
list as the phone layout. Conversions are now in that list, so phones can reach
them too.
- `RlButton` labels do not wrap. The title keeps a minimum width of 16rem.
