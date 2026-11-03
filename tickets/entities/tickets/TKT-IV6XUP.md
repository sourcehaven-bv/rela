---
id: TKT-IV6XUP
type: ticket
title: Skip the busy safety replay where no other process can write
kind: chore
priority: low
effort: s
status: backlog
---

## Description

TKT-W5ZSYJ adds busy boards to the inbox safety replay on every backend. On
single-process backends (sqlite) no other process can move a session\, so that
replay can never find a change. Gate both the inbox and busy safety replays on
whether another writer can exist; consider it together with TKT-YK30D1.
