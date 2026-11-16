---
id: TKT-IAQO7HG
type: ticket
title: 'Flows: run working state on git-shared file projects'
kind: enhancement
priority: medium
effort: l
status: backlog
---

## Description

On the file backend a run's `_flow-run` entity is a markdown file under
`entities/_flow-runs/`, committed with the data, while its working state is in
the node-local `.rela/flow-runs.json`. A project shared through git and worked
on in several checkouts (the tickets project) would see runs that cannot advance
in every other checkout, and an open run blocks renaming or converting its
subject on every surface (blocker R3 of the decision on TKT-XB4M29R).

Decide and implement one of: keep the working state with the data on the file
backend; refuse or warn about flows on a git-shared file project; or keep the
runs out of the committed tree. Record the choice as a decision.

## Acceptance

- A run started in one checkout of a file project can advance, or be closed
cleanly, in another checkout after a git pull, or rela refuses the setup with a
clear message.
