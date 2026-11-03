---
id: TKT-YJA7KT
type: ticket
title: Add web URLs to MCP entity results
kind: enhancement
priority: medium
effort: s
started: "2026-10-24"
completed: "2026-10-24"
status: done
---

## Description

MCP results name entities by id only. An agent cannot give the user a link to
the entity in the web app; it has to guess the host and the route.

Add a `url` field to every entity the MCP server returns: show, create, update,
list, search, mentioned_by, relation targets, trace and path nodes, and pile
items. The URL is `<base>/entity/<type>/<id>`, with `<id>@<face>` for a faced
row.

The base is the app origin the request reached (scheme from TLS or
`X-Forwarded-Proto`, host from `Host`), the same rule the calendar feed uses for
its deep links. Stdio MCP has no request and therefore no base; it omits `url`.
The `/s/<space>` prefix is left out: a type can sit in several spaces, and the
unprefixed route works in every space.
