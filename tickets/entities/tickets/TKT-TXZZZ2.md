---
id: TKT-TXZZZ2
type: ticket
title: Authored confirm and style for session decisions
kind: enhancement
priority: low
effort: s
status: backlog
---

## Description

Add an optional `confirm:` and `style:` on a session transition (`on.<type>`),
served in the schema descriptor. They override the derived "confirm when moving
into a final state" rule of the one-click decision buttons from TKT-T9G1AE, for
example to confirm an Allow that runs a tool, or to show Abandon in danger
style.
