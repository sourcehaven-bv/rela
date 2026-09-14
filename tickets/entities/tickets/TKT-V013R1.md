---
id: TKT-V013R1
type: ticket
title: Open several projects at once in rela-desktop
kind: enhancement
priority: medium
effort: l
status: backlog
---

Mount each open project at `/p/<id>/` so File > Open Project adds a project in
its own window rather than replacing the one the user is looking at, and switch
between them from the sidebar.

Groundwork for the same switcher on `rela-server`, where a user picks between
several configs from a pull-down. Desktop first, because it avoids the two
riskiest parts: the app-iframe CSP coupling, and `internal/lua/urls.go`, whose
`url.entity()`/`url.form()` output is an operator-facing contract.

## Identity

A hash of the absolute path, not a slug of the folder name. Desktop projects
arrive through a file picker from anywhere on disk, so `~/acme/tickets` and
`~/globex/tickets` are both legitimately "tickets" and would collide. Borne out
in testing: two copies of this repo's own tickets project both report the name
"Rela Development", and are distinguished only by root. A server, where an
operator configures the list deliberately, can afford readable slugs — the
scheme is the only thing that differs.

## Why the SPA reads its base at runtime

`import.meta.env.BASE_URL` is substituted at build time, so one bundle could
only ever serve one prefix — and that bundle is embedded in the Go binary.
Instead the shell carries `<meta name="rela-base">` and `relaBase()`/`apiUrl()`
read it at boot. Unprefixed both are identity, which is why the existing
frontend tests pass unchanged.

A meta tag rather than an inline script: the SPA shell has no CSP today, but
`router.go` records that as a property that could lapse, and a meta tag needs no
`unsafe-inline`.

## What did NOT need changing

Sidebar and document hrefs. They are fed to `RouterLink` and `router.push`, and
vue-router prepends the history base itself — verified: `resolve('/list/x')`
under base `/p/abc/` gives `/p/abc/list/x`, and `route.path` comes back
stripped. Prefixing them server-side would have doubled the prefix. This was
originally recorded here as a gap; it was not one.

## What did

URLs the browser fetches without the router seeing them: attachment hrefs (`<img
src>`, `<a download>`, and the DELETE) and the logo. Prefixed client-side, where
the base is known — the server builds them deep in `dataentry`, which has no
business knowing about desktop mounting. `apiUrl` only prepends and never
re-encodes, so the single-escaper invariant `attachments.ts` documents still
holds.

Middle- and cmd-click. The desktop intercepts these before the browser, so the
fix is in `OpenWindow`: the injected script sends the base its page was served
under, and the route is mounted under it. Otherwise a link clicked in one
project's window opens against whichever project happens to be active.

## Still open

`rela-server`, where the same mechanism needs a slug scheme, the app-iframe CSP
moved in lockstep, and `internal/lua/urls.go` handled — operator scripts already
emit `/entity/...` and would silently lose the prefix.
