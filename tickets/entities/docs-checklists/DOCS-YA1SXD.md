---
id: DOCS-YA1SXD
type: docs-checklist
title: 'Docs: mail.render binding, per-message lang, and the dark-mode posture'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Package doc comments updated
- [x] Exported symbols documented
- [x] Non-obvious decisions explained with rationale

- `internal/lua/mailrender.go` — file-level doc covers why the binding exists,
why `lua → mailrender` does not invert the `mail → lua` arrow, why there is
deliberately no raw-HTML/CSS field, and why every failure raises.
- `internal/mailrender/template.go` — three new sections on the template doc:
padding/margin live on table cells (with the list exception spelled out as an
exception), dark mode is defensive and the `color-scheme` meta tag is a trap,
and `.prose` scopes the markdown table styling.
- `internal/mailrender/mailrender.go` — `Message.Lang` documents why language is
on `Message` and not `Options`; `ValidateLang` documents why it rejects rather
than escapes, and why it is a shape check rather than a registry lookup.
- `internal/mailrender/compat_test.go` — documents that the dataset is a FLOOR
not a source of truth, and carries a runnable recipe for refreshing the vendored
fixture.
- `BaseURLCarrier` cites `RecipientPolicyCarrier` rather than restating its
rationale, and records the one place the two genuinely differ (an absent
recipient policy must deny; an absent base URL is safely unknown). This was an
advisory `duplication` finding from `just comment-report` that the diff
introduced — fixed rather than suppressed.

## Project Documentation

- [x] `CLAUDE.md` updated with new patterns or conventions

Four invariants added to the existing mail bullet, each stating the failure mode
rather than just the rule:

- `lua` may import `mailrender`, and why that does not invert `mail → lua`.
- Email CSS is not web CSS — why headings are tables and gaps are spacer rows,
plus a pointer to the dataset-driven guard.
- Dark mode is defensive and `<meta name="color-scheme">` is deliberately absent
(the Apple Mail trap), with the three client tiers.
- Language belongs on `Message`, never on `Options`, because a `Renderer` is
built once per deployment.

## External Documentation

- [x] `docs/mail.md` updated
- [x] `docs/lua-scripting.md` updated
- [x] Examples provided and verified

- **`docs/mail.md`** — new "Rendering a branded message from a script" section
with a worked Dutch example, a field table, and the five things worth knowing
(markdown vs. escaped cells, vetted links, no raw-HTML field, works without mail
configured, malformed calls raise). Added `lang:` to the declarative template
example plus a paragraph on why it is per-template and that it labels rather
than translates.
- **`docs/lua-scripting.md`** — `mail.render` documented in the Mail Functions
section alongside `mail.send`.
- **Stale text corrected while there:** `docs/mail.md` still said "**Sending
from Lua** will add a `mail.send` binding" — future tense for something that
shipped in TKT-DS1CR6. Now points at `mail.render` for what the declarative form
does not cover.
- Examples are not hand-written prose: the Dutch example is the same shape as
the message rendered end-to-end during manual verification, which produced the
screenshots reviewed in the implementation checklist.

## Not applicable

- [x] ~~`docs/metamodel.md`~~ (N/A: no metamodel change)
- [x] ~~`docs/cli-reference.md`~~ (N/A: no new or changed command)
- [x] ~~`docs/data-entry.md`~~ (N/A: no UI change)
- [x] ~~`README.md`~~ (N/A: no project-level change)
