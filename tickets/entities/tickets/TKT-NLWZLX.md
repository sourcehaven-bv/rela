---
id: TKT-NLWZLX
type: ticket
title: faces.<name>.messages.notice — text on a face regardless of writability
kind: enhancement
priority: medium
effort: s
status: done
---

`FaceMessages` has one key, `read_only`, gated on the reader NOT being able to
write. A draft-like face is writable by definition, so no config can put a
warning on it. An operator needs a banner on such a page saying the document is
not yet in force.

## Problem

An ISMS-style deployment needs one sentence on a **draft** detail page: this
text is not yet adopted. There is no config that produces it.

The three chrome keys each miss for a different reason:

| Key | Why it cannot |
| --- | --- |
| `faces.<name>.messages.read_only` | Gated on `!mayUpdate` (`EntityDetail.vue:949`). A draft face is writable, so it renders nothing. |
| `worlds.<name>.banner` | Per-world, not per-face. A world that falls back from adopted to draft serves both, so the banner marks both. |
| `worlds.<name>.messages.stand_in` | Renders only through `WorldBadge`, used by lists, kanban and the relation picker. `EntityDetail` never uses it for this. |

Today's chrome can only explain why a reader *cannot* edit. What is missing is
text on a page where they *can*. The unadopted page is the one that carries
risk: a reader who acts on it follows something the organisation has not agreed.
It is currently the unmarked one.

## Request

A second key on `FaceMessages`, shown whenever that face is on screen, whatever
`_actions` says:

```yaml
entities:
  beleid:
    faces:
      concept:
        label: Concept
        messages:
          notice: 'Let op: dit is een concept en nog niet vastgesteld.'
      vastgesteld:
        label: Vastgesteld
        # no messages: the adopted version needs no explanation
```

Same rules as the rest of the face chrome: plain text, the existing
`ChromePlaceholders` allowlist (`{face}`, `{bare_face}`, `{world}`, `{title}`),
undeclared renders nothing, no rela-authored fallback.

`notice` and `read_only` are independent and may both appear on one face. Where
both apply, `notice` renders first: it is about the document, `read_only` is
about the reader.

## Why not widen read_only

Dropping its `mayUpdate` guard would change what `read_only` means for every
operator who already wrote one. The two sentences say different things: "you may
not edit this" versus "this is not in force". An editor holding `update` on the
draft needs the second and not the first.

## Surfaces (verified on `develop` @ f1d57608)

- **metamodel**: `FaceMessages.Notice` (`internal/metamodel/types.go:387`,
which today declares `ReadOnly` as its only field). No new validation beyond the
placeholder allowlist; the key names no other object.
- **wire**: `v1.FaceMessages` (`internal/apiwire/v1/responses.go:487`) gains
`notice`; `faceMessagesWire` (`internal/dataentry/schemaworlds.go:173`) copies
it. That function compares against the zero struct to decide whether to emit the
block, so adding a field keeps "nothing declared omits the block" working
unchanged.
- **SPA**: `frontend/src/types/schema.ts:99` widens
`messages?: { read_only?: string }`; `EntityDetail` gains a `noticeNote`
computed beside `readOnlyNote` (`EntityDetail.vue:948`) and the second
`WorldBanner` (`EntityDetail.vue:1489`) adds it to its `v-if` and slot. No new
component.

## One resolution detail not in the request

`readOnlyNote` returns `''` when `worldAbsent` is true, and its `WorldBanner` is
already inside `v-if="!worldAbsent"` — the absent case has its own banner above
it. `notice` must follow the same rule: there is no face on screen when the
world resolves to none, so a per-face notice has nothing to be about. Decide and
pin this in a test rather than inheriting it by accident.

## Not done: the edit form

`DynamicForm` is out of scope, and the reason is not the one it first looks
like. Its `notEditableNote` consumes `read_only`, so an obvious reading is
"extend that call site too" — but that note renders only in the `notEditable`
branch (`_actions.update === false`), which a writable draft never reaches.

The stronger reason is that **the edit form has no banner surface at all**:
there is not one `WorldBanner` in the file. `notEditableNote` is the body text
of a refusal screen, not chrome. Putting a notice there means designing a new
surface (where does a standing note live on a form, above the fields or beside
the title?), not extending this feature.

There is a real gap here — an operator editing a draft sees no "not yet in
force" reminder on the very form where they are changing it, arguably where it
matters most. That is a separate ticket with a design question, deliberately not
answered by this one.
