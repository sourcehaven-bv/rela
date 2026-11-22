---
id: BUG-E4XWNGR
type: bug
title: List title link colour without link:, nested and card sections on phones
description: A list title whose column has no link field renders as plain text although it links to the entity, a nested section cuts off its column values on a phone, and a cards section can be wider than a small phone.
priority: medium
effort: s
why1: BUG-6WQ1U8S gave the title link the accent colour only when a column sets link. The nested row kept its values in one line that cannot shrink, inside a tree that clips overflow. The cards grid had a fixed 300px minimum column.
why2: BUG-6WQ1U8S fixed the tables on the entity page and treated the link field as the sign of a link, but the title links to the entity in either case.
why3: The other section displays were not checked at phone width when the tables were.
why4: No e2e fixture has a nested section, and the e2e mobile spec covers lists, dashboards and table sections only.
why5: Each section display carries its own layout, with no shared rule for narrow widths.
prevention: A unit test pins the title link class with and without link. The nested and card layouts are container and grid CSS, checked in a browser at 375px and on desktop.
started: "2026-11-22"
completed: "2026-11-22"
status: done
---

## Description

Found in the saas-dev config pass after BUG-6WQ1U8S.

- A list title whose column has no `link:` links to the entity but renders in
the text colour, unlike dashboard cards.
- On a phone, a `display: nested` section cuts off the column values at the
right edge.
- On a phone narrower than a card, a `display: cards` section is wider than the
page.

## Acceptance Criteria

- [x] The list title takes the link colour when the row links to an entity, with or without `link:`.
- [x] A `display: nested` section 480 pixels wide or narrower puts its column values on their own line, where they wrap; a long title stays beside the twisty.
- [x] A card shrinks to fit a section narrower than 300 pixels.
- [x] Wider sections keep their layout.
- [x] The behaviour is documented.
