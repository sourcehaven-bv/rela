---
id: BUG-NYI1XNO
type: bug
title: Rendered mail ignores the app palette and logo, and its text part drops email addresses
description: Lua mail.render and scheduled mail always use rela's default accent and no logo, whatever the app's palette and app.logo say; the plain-text part drops a bare email address and the address of a mailto link.
priority: medium
effort: s
why1: mail.render and the scheduled-mail path built the renderer with only a base URL, so it had no palette and no logo; the text walker kept an autolink only when it was an http link, so email autolinks and mailto links lost their address.
why2: The palette and the logo live in the data-entry layer (data-entry.yaml, the Settings palette in the state store), which the Lua runtime and the scheduler did not reach.
why3: The mail docs described branding from the palette and an embedded logo, but no caller supplied either, and the renderer's own tests set the options directly.
why4: No test rendered mail through a real caller with an app palette, and the text-part tests used only http links.
why5: Branding had no owner between the app's look and the mail renderer, so each caller had to wire it and none did.
prevention: mailbrand.Source owns the app's look for mail and is wired into every Lua runtime and the scheduled-mail path; tests pin the accent and logo through mail.render and a scheduled template, and the text part's handling of bare, autolinked and mailto addresses.
started: "2026-11-22"
completed: "2026-11-22"
status: done
---

## Description

Found in the saas-dev config pass.

- Lua `mail.render` builds its renderer with only a base URL. Mail buttons and
the top bar always use rela's default #4772fb, and no logo shows.
- The plain-text part of a rendered mail drops a bare email address, because
autolinks are kept only when they are http links. A `mailto:` link loses its
address too.

## Acceptance Criteria

- [x] mail.render and scheduled mail use the accent of the app's palette (data-entry.yaml, with the Settings palette over it).
- [x] app.logo shows as the mail logo, made absolute with the base URL; an SVG or a missing logo gives no logo.
- [x] Without a palette, or when the palette cannot be read, mail uses the defaults.
- [x] The text part keeps bare, autolinked and mailto email addresses.
- [x] The behaviour is documented.
