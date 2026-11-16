---
id: TKT-62NRDMJ
type: ticket
title: 'claude-workflow template: name flows.yaml triggers and the create-in-status rule'
kind: docs
priority: medium
effort: xs
status: backlog
---

## Description

`.claude/commands/ticket.md`, `bug.md` and `implement.md` and the workflow block
of CLAUDE.md are `@managed: claude-workflow v1`. They still say "Automation
creates" the checklists. Since #2350 a `triggers:` entry in `tickets/flows.yaml`
does that. Local edits are overwritten, so the change has to land in the
claude-workflow template.

- Say that a flows.yaml trigger creates the checklist.
- Add: create a ticket or bug in `backlog` or `ready` and then move the
status, because a status trigger does not fire on create.

## Acceptance

- The template and the synced files in this repo carry both points.
