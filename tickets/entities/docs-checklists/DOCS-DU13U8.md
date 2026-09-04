---
id: DOCS-DU13U8
type: docs-checklist
title: 'Docs: Make list rows and kanban cards behave as real links'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Exported functions/types have godoc
- [x] Non-obvious decisions explained in comments
- [x] Package docs updated if package purpose changed

No Go code changed, so no godoc. The Vue side carries comments at the three
places where a later reader would otherwise undo a deliberate choice — each is
also recorded as a review-response:

- **`rowCellWrapper`** documents that the click handler must be bound as
`:onClick` rather than written `@click.stop`. Vue resolves `.stop` at COMPILE
time, so on a wrapper shared by every column it halts propagation for all of
them and the plain columns swallow clicks the row handler needs. This was a real
regression during implementation (RR-BWZ0TR); the comment is what stops it
recurring the next time someone tidies the template.
- **`encodeRoutePath` / `encodeRouteQuery`** document why neither
`URLSearchParams` nor `router.resolve()` is used: the former writes a space as
`+` where vue-router writes `%20`, and an unencoded `?` in a path splits into a
query when the browser parses an href, sending a Cmd+click somewhere a plain
click never goes (RR-M8XNQE).
- **`.row-link` CSS** documents why it is `display: inline` and NOT
`display: contents`, while the neighbouring `.row-cell` is: WebKit drops an
element's implicit ARIA role under `display: contents`, which would unexpose the
anchor as a link to VoiceOver — losing the semantics the change exists to add.
Without the comment the two rules look inconsistent and invite a "cleanup"
(RR-P4TDVL).

The template comments also record why the anchor sits where it does: a `<tr>`
cannot be an anchor, wrapping every cell would put links around checkboxes and
delete buttons, and on kanban an anchor around the draggable card would make the
browser start a link drag instead of the board's own drag-and-drop.

## Project Documentation

- [x] ~~CLAUDE.md updated with new patterns~~ (N/A: introduces no new
architectural pattern or rule — it changes markup on four existing render sites)
- [x] ~~docs/ updated for changed behaviour~~ (N/A: see Rationale below)
- [x] ~~Architecture docs updated~~ (N/A: no package boundary, dependency or
wiring-contract change; frontend-only, `arch-lint` clean)

## External Documentation

- [x] ~~README updated~~ (N/A: no project-level change)
- [x] ~~CLI reference updated~~ (N/A: no new or changed command or flag)
- [x] ~~API docs updated~~ (N/A: no HTTP/MCP surface change)

## Rationale for N/A

`docs/data-entry.md` is the doc that covers lists and kanban boards, and it
needs no change.

Where a click goes is unchanged — same routes, same query, same prev/next and
back-button context. What changes is that the browser can now *see* those
destinations, so right-click "Open Link in New Tab", Cmd/Ctrl+click and
middle-click work.

Those are standard hyperlink affordances that users already expect from anything
that looks clickable; their absence was the bug, and it was never documented as
behaviour. Writing "rows are now links" into the docs would be documenting the
absence of a defect, which goes stale the moment anyone reads it as a feature
flag.

No configuration surface is added: no new `data-entry.yaml` key, no per-list or
per-board opt-in. A list or board author writes exactly the same config as
before.

The one operator-visible consequence weighed and rejected as a doc change: an
`edit_form`-configured board now exposes that form URL in the card's `href`.
That URL was already reachable by clicking the card, and the route enforces the
same server-side authorization either way, so nothing becomes visible that was
not before.
