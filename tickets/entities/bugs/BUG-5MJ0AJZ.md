---
id: BUG-5MJ0AJZ
type: bug
title: Open step scrolls the app shell frame and hides the top bar
description: Bringing a run card into view with ?run= scrolls the clipping app shell frame as well as the page.
priority: medium
effort: xs
why1: 'focusRunCard called scrollIntoView on the run card; scrollIntoView scrolls every ancestor scroll container, and the app shell frame was one because overflow: hidden makes a box a scroll container.'
why2: 'The frame used overflow: hidden to clip its content; overflow: clip clips the same way without being scrollable, but was not used.'
why3: The unit test stubbed scrollIntoView and only checked that it was called; jsdom has no layout, and the app shell was not part of the test.
why4: The panel was checked by hand on a desktop page that did not need the frame to move.
why5: No test pins which container may scroll when the app brings content into view.
prevention: appShellFrame.test.ts fails when the frame's overflow is anything but clip, and pins the card's scroll margin below the sticky top bar.
started: "2026-11-19"
completed: "2026-11-19"
status: done
---

## Description

In the harbourmaster portal, the next-action "Open step" link opens the company
page with `?run=<run>`. The page then scrolls the run card into view with
`scrollIntoView`, which also scrolls `.rl-app-shell__frame` (overflow hidden).
The layout shifts up and leaves a blank band; on mobile the top bar disappears.

## Expected

Only the page's scroll container scrolls; the shell stays in place.
