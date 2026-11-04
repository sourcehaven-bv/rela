---
id: TKT-HG4PO94
type: ticket
title: Add a note to a one-click session decision
kind: enhancement
priority: medium
status: backlog
---

## Description

A one-click decision posts with no properties, also when its type has optional
fields (Approve plan with an optional `note`, Allow with `note`, Deny with
`reason` in the agent runner POC). Until TKT-A0RTC52 the composer's type picker
was the web path to those fields. TKT-A0RTC52 takes decision types out of the
picker so each decision shows once, which leaves no general way to add a note to
a one-click decision.

Decide how a person adds optional fields to a one-click decision. Options
include a secondary "with a note" action on the button, a `decision_button` key
that makes the button open the form, or relying on preset buttons with `compose:
true`. This is a design decision outside the scope of TKT-A0RTC52.
