---
id: TKT-RROOP1
type: ticket
title: 'Co-editing: merge external body edits into an open room'
kind: enhancement
priority: medium
effort: m
status: backlog
---

## Description

While a co-editing room is open, merge edits that arrive outside it (Lua,
webhook `append_section`, history restore, CalDAV, checkbox toggles) into the
shared document instead of overwriting them (RES-L4FVT0 phase 2).

## Approach

The server sees the store event, compares the stored content hash with the last
content the room saved, and on a mismatch sends the new markdown to one client,
which applies it with y-prosemirror `updateYFragment`. Collab saves carry a
content precondition so an in-flight autosave cannot overwrite the external edit
before it is merged.
