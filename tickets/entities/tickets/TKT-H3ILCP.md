---
id: TKT-H3ILCP
type: ticket
title: Suggest edits with full editing (track changes)
kind: enhancement
priority: medium
status: backlog
---

## Description

Let a user edit the whole body freely, with full formatting, and post the edit
as suggestions instead of saving it, as track changes does in Word and
LibreOffice. Today a suggestion is a replacement typed into a box for one
selected block.

## Design (from the Storybook mockup)

The mockup is in `frontend/packages/rela-components`, under "Mockups / Track
changes".

- **Entry point.** A pen beside the "Description" heading opens a menu with **Edit** ("Changes are saved directly") and **Suggest changes** ("Others review them first"). The user picks the mode for each edit. Clicking the text still edits directly. We compared this with a sticky Edit | Suggest switch beside the pen and chose the menu: the switch is visible on every page view, and a sticky mode can post suggestions when the user meant to save.
- **Comment-only readers.** A reader who may comment but not edit gets a single **Suggest changes** button and no menu. A click on the text also opens the editor in suggesting mode.
- **Suggesting.** The normal Milkdown editor opens with a green edge and a bar with **Post N suggestions**. On posting, the edit is diffed against the stored body and each hunk becomes one suggestion. The suggestions from one post share a change-set id.
- **Review.** Changes are drawn in the body: added text underlined in green, removed text struck through in red, formatting changes dotted. A pill beside the heading counts the open suggestions. Clicking a change opens a card with **Accept**, **Reject** and **Reply**, which then moves on to the next open change. Accept all and Reject all apply to one change set.

## Diff

- Align lines first, so an untouched paragraph never produces a change.
- Diff a changed line word by word, keeping the markdown prefix (`## `, `- `) out of the comparison.
- Merge hunks that are separated by only one short word.
- Milkdown re-serializes markdown, so compare blocks semantically (`isSemanticallyEqual`) before diffing. A block that only changed spelling must produce no suggestion. Spike this first.

## Rules that change

- A suggestion may replace a range spanning several blocks. TKT-U32AUB's segments already handle highlighting such a range.
- An insertion needs an anchor: widen it to the neighbouring word, or store a point anchor.

## Open questions

- Suggestions from several people that overlap. The mockup only shows the newest change set.
- The editor opens on the stored text without open suggestions, so a second reviewer does not see the first one's changes while typing.
