---
id: TKT-2F1AUZ
type: ticket
title: Follow a retired id to its successor on the entity detail page
kind: enhancement
priority: medium
effort: m
status: done
---

## Description

After a rename or a type conversion, the old id is retired. On the database
backends (pgstore, sqlitestore) the store keeps a mapping from the retired id to
its successor (`store.RetiredIDs`), and mentions in bodies already resolve
through it at read time (`visibility.Resolver.ResolveMentions`).

Hyperlinks do not. Opening `/entity/topic/TOPIC-AOWJ` after converting that
topic to `INIT-P2HD` shows "entry entity not found". Bookmarks, links in mail,
chat and other tools all break on every conversion. Atlas is about to convert
all its topics (atlas TASK-WVDXT), so every link to a topic breaks.

## Goal

When a detail page asks for an id that a rename or conversion retired, and the
principal may read the entity that now carries it, the server tells the client
where the entity is now, and the SPA replaces the URL with the new address. A
principal who may not read the successor gets the ordinary not-found, so the
redirect discloses nothing.

## Out of scope

- fsstore / memstore: they rewrite mention text on rename and keep no
retired-id mapping, so there is nothing to follow.
- MCP `show_entity` and the CLI (possible follow-up).
