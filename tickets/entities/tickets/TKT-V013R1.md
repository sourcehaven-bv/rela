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
its own window rather than replacing the one the user is looking at.

Groundwork for the same switcher on `rela-server`, where a user picks between
several configs from a pull-down. Desktop first, because it avoids the two
riskiest parts: the app-iframe CSP coupling, and `internal/lua/urls.go`, whose
`url.entity()`/`url.form()` output is an operator-facing contract.

## Identity

A hash of the absolute path, not a slug of the folder name. Desktop projects
arrive through a file picker from anywhere on disk, so `~/acme/tickets` and
`~/globex/tickets` are both legitimately "tickets" and would collide. A server,
where an operator configures the list deliberately, can afford readable slugs —
the scheme is the only thing that would differ.

## Why the SPA reads its base at runtime

`import.meta.env.BASE_URL` is substituted at build time, so one bundle could
only ever serve one prefix — and that bundle is embedded in the Go binary.
Instead the shell carries `<meta name="rela-base">` and `relaBase()`/`apiUrl()`
read it at boot. Unprefixed both are identity, which is why the existing 2344
frontend tests pass unchanged.

A meta tag rather than an inline script: the SPA shell has no CSP today, but
`router.go` records that as a property that could lapse, and a meta tag needs no
`unsafe-inline`.

## Known gaps

Server-emitted absolute paths are not yet prefixed — sidebar hrefs
(`views_handler.go`), attachment hrefs (`affordances.go`), the logo URL, and the
document link rewriter. They are correct at the root, so the active project
works; a second window will show unprefixed links until that lands.

`useDocumentClicks` has no free lunch: it does `new URL(href, origin)` then
`router.push(url.pathname)`, which double-prefixes under a router base. Emitting
unprefixed and stripping explicitly is the fix, but middle-click — deliberately
passed to the browser — then needs the prefix put back.
