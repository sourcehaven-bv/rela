---
id: TKT-IFX7KU2
type: ticket
title: Give data-entry runtimes the in-memory test seams
kind: chore
priority: medium
status: backlog
---

## Description

`appbuild.NewInMemory` applies the clock, mail, HTTP, AI and output-dir seams to
the automation, scheduler and cascade runtimes. The data-entry action and
validation runtimes are built outside appbuild and do not get them (RR-UI7PDOA
on TKT-NRNDGZR). A data-entry app mounted on an in-memory world would read real
secrets and `ai.yaml`, and send real mail.

Until this is done, do not mount the data-entry app on a `NewInMemory` world.

## Acceptance

- Data-entry runtimes built for an in-memory world use the same seams as the automation runtimes.
- A test shows a data-entry action on an in-memory world cannot reach the network or real mail.
