---
id: BUG-7JY30MC
type: bug
title: On mobile the hamburger overlaps the next-action banner
description: On a phone the raised menu button covers the top 8px of the next-action banner below it.
priority: medium
effort: xs
why1: 'The hamburger is sticky with top: 8px. A sticky offset counts from the pane''s content edge. The button is drawn 8px below its own row and covers the top 8px of the next element.'
why2: The button had no bottom margin. The next-action banner started right where the button's row ended.
why3: The mobile layout tests measured the button against the notch and the pane edge. None measured it against the content below it.
why4: The next-action banner is the first element on a page only when a banner next action is configured. The test fixture has none.
why5: Layout tests check one element against the frame instead of the spacing between neighbours.
prevention: An e2e test on the mobile project measures at least 8px between the hamburger and a next-action banner.
started: "2026-11-19"
completed: "2026-11-19"
status: done
---

## Description

On a phone, an entity page with a banner next action shows the raised menu
button over the top edge of the banner. The two touch, and a tap near the button
can land on the banner.

## Expected

The button and the banner have clear space between them.
