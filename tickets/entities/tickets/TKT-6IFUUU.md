---
id: TKT-6IFUUU
type: ticket
title: Filter lists on latest_by roll-up values
kind: enhancement
priority: low
effort: l
status: backlog
---

## Description

Follow-up of TKT-6I2OPG. A list filter on a `latest_by` roll-up (for example
`laatste_testresultaat == gefaald`) is refused today. Pushing it down needs a
lateral/ranked subquery per backend that keeps the reader's gate and field
visibility.
