---
id: TKT-DLATUI
type: ticket
title: Read state, actor and reason on declared pile items; unread badge and expiry
kind: enhancement
priority: medium
effort: l
started: "2026-11-03"
completed: "2026-11-03"
status: done
---

## Description

Give items on declared piles read state, an actor and a reason, so a pile can
act as a notification inbox. Add mark-read (manual and automatic), an unread
badge, and expiry of read items.

```yaml
# schema.yaml
piles:
  notifications:
    name: Notifications
    icon: bell
    read_state: true
    auto_read: on_open      # off (default) | on_open
    clear_read_after: 7d
    expire_after: 30d       # optional, also drops unread items
    on_repeat: bump
    reasons:
      completed: "completed"
      assigned: "assigned you"
```

## Design

- The read flag lives on the pile item (`read_at`), not in a service that tracks all entity reads. Read state is about the notification, not the entity. A global read log would write on every page view and is sensitive data about who looked at what.
- Item fields: `read_at` (empty = unread), `actor` (person ref of who caused the push), `reason` (a key from the pile's `reasons:`). The reason label is operator config, never interpolated from entity values. Interpolating at push time would show the recipient values that field-level `visible:` redaction hides from them.
- The actor is resolved through the reader's visibility gate. A hidden actor shows as "someone".
- A bump (repeat push) clears `read_at`.
- Expiry: reads hide items past `clear_read_after` / `expire_after`, so a read never writes. The next write to the pile deletes them in the same atomic step.
- Eviction on a full declared pile: read items go first, then the oldest. This applies to pushes by others too, because a declared pile is system-owned, not hand-collected.

## Scope

- Store: new item fields in kvpiles and pgpiles; `pilestest` conformance for read, bump, expiry and eviction order.
- API: mark read for refs, mark all read, unread count per declared pile.
- SPA: unread badge on the `pile:` navigation entry; per-row "mark read" and "mark all read" in the pile panel; read items shown dimmed.
- Auto-read: on opening an entity, the SPA calls mark-read only when the entity is in its unread set, so a normal page view causes no write. Never a side effect of the entity GET.
- Show actor and reason label on each item.

## Depends on

The declared-piles ticket.
