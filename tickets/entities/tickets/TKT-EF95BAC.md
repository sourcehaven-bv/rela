---
id: TKT-EF95BAC
type: ticket
title: Enforce required_when on the server
kind: enhancement
priority: medium
effort: s
tags: security
status: backlog
---

## Description

`required_when` is enforced only by the SPA; the server only lints it. Any other
client, including a future native app or TUI, can submit a form without the
field. Enforce it on the server when a form submission names the form.

Part of FEAT-QRI3CFR (step 3.3).
