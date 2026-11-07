---
id: TKT-H0TFDWU
type: ticket
title: 'World switcher: labels; hidden worlds; header or none; per-record toggle'
kind: enhancement
priority: medium
effort: m
started: "2026-11-07"
completed: "2026-11-07"
status: done
---

## Description

The world switcher ("World [name ⌄]") appears in the sidebar header of every
project that declares two or more worlds. It lists raw world names and shows on
every page. The operator cannot hide it, label it or move it.

Worlds are often wiring, not something a reader chooses. OpenVWR v2 (a GDPR
processing register) declares `werk`, `vastgesteld` and `publiek`: two faces of
a record, plus a feed world for its public site. Its users read a register, not
worlds. For them the switcher is noise ("Wereld [werk]"), and it offers the feed
world as if it were a view. OpenVWR v2 now hides the switcher with
`custom/custom.css` (`.world-switcher { display: none }`). That selector has no
contract under FEAT-MD4N6Z.

The concept is useful where readers do move between worlds, such as a handbook's
draft and published text, or languages. It needs presentation that fits.

## Proposal

1. **Labels.** Add `label:` to a world in `schema.yaml`. The switcher, the
banner placeholders and the "{face} bekijken" affordances use it. The raw name
stays in the URL.
2. **Opt-in placement.** Add `app.world_switcher:` to `data-entry.yaml` with
the values `header`, `detail` and `none`. Choose a default: `none` is safest for
projects that use worlds as wiring, and `header` keeps today's behaviour.
3. **Hidden worlds.** Add `hidden: true` on a world. A hidden world stays out
of the switcher but is still selectable through `?world=` and the API. This is
for feed worlds such as `publiek`.
4. **Control.** For two or three visible worlds, use a segmented toggle
instead of a labelled dropdown. The world banner already says where the reader
is.
5. **On a record page.** With `detail`, the toggle switches that record's
face. It offers only the faces the record has, through `worldForFace`. This
replaces "Vastgesteld bekijken" / "Concept bekijken" in the more-actions menu
with something visible.
6. **Optional follow-up.** Add `world:` on a list or navigation entry, so that
a "Published versions" list opens in that world without a switcher.

## Acceptance

- A project with worlds and `world_switcher: none` shows no switcher; the
face affordances on detail pages still work.
- World labels appear wherever a world name was shown to a reader.
- A `hidden` world is absent from the switcher but `?world=<name>` still
resolves it.
- `docs/data-entry.md` and `docs/content-states.md` describe the new keys.
