---
id: TKT-6V8K8I
type: ticket
title: 'Forget a reader: erase one principal''s read markers on request'
kind: enhancement
priority: medium
effort: s
status: backlog
---

## Description

Read markers (TKT-2B2AIA) record when a named user read each channel and are
kept until the channel goes, also after the user entity is removed. A new user
who later gets the same id (reused login or email) inherits those read positions
(security review RR-RUL4TA).

Add a `channels.Store` method that deletes every marker of one reader (user and
org) on all backends, pinned by channelstest. Call it when the user entity is
deleted, and expose it to operators through a CLI command and an admin-only API.

Link it from the privacy and retention notes in docs/realtime.md, so an erasure
request covers markers along with the other per-user data.
