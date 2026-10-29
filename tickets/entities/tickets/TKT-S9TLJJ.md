---
id: TKT-S9TLJJ
type: ticket
title: Authored one-click presets for session transition entries
kind: enhancement
priority: low
effort: s
status: backlog
---

## Description

A session transition entry declares `button: { label, props }`, and the web app
shows one one-click decision per entry, for example Approve and Revise from one
enum-typed "plan decision" message type. The button posts the preset props with
`expect_generation`. Until then, two message types remain the better choice for
one-click buttons. Follows TKT-FPXTQJ and TKT-T9G1AE; plan together with
TKT-03FQPD and TKT-TXZZZ2.
