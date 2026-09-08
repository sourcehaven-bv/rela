---
id: TKT-5SZG2L
type: ticket
title: 'World chrome speaks the operator''s words or stays silent: messages, on_absent redirect, copy on_success'
kind: enhancement
priority: medium
effort: m
status: done
---

Atlas worlds issue 6 (`atlas-world/docs/rela-worlds-issues.md`). After
TKT-SLFURL the web app still shows rela's own vocabulary to readers: the
"read-only face" note, the "no face in this world" banner and its "Go to
Concept" button, the list/board projection notes, the form's face refusal, the
stand-in badge, and the copy toast. "Face", "world", "bare", "default" are the
storage model leaking onto the screen. A reader who cannot edit an adopted
policy needs no explanation, any more than a reader without update permission
does.

## Decision (2026-09-05)

**Silence by default, operator text where the operator wants it, behaviour keys
for the two things that are not text.** No rela-authored fallback sentence
anywhere; an undeclared message renders nothing.

```yaml
worlds:
  actueel:
    select: [vastgesteld, concept]
    otherwise: default
    banner: 'Actueel beleid'              # existing
    messages:
      absent: 'Dit item heeft nog geen vastgestelde versie.'   # detail page, no face in this world
      projection: 'Alleen vastgestelde items.'                 # list and board note
      stand_in: '{face}'                                       # badge on a stand-in row/card
    on_absent:
      redirect: default                   # a declared world; lands the reader there instead of the note

entities:
  beleid:
    faces:
      vastgesteld:
        messages:
          read_only: 'Dit is de vastgestelde versie. Bewerken doe je in het concept.'  # detail note + form refusal

copies:
  vaststellen:
    on_success:
      message: 'Beleid vastgesteld.'      # toast; default is the copy's own label
      landing: written                    # written (default) | stay | { world: name } | { face: name }
```

Placeholders, substituted client-side: `{face}` (served face label),
`{bare_face}` (bare face label), `{world}` (world name), `{title}` (entity
title).

What goes: the read-only note default, the absent banner default and its "Go to"
button (the face switcher already reaches the bare face by address), the
list/board note defaults, the badge's default text and tooltip, and the "Created
the X face" toast (default becomes the copy label alone).

## Surfaces

- metamodel: `WorldDef.Messages`, `WorldDef.OnAbsent`, `FaceDef.Messages`,
`CopyDef.OnSuccess`; validation: `on_absent.redirect` and `landing.world` must
name declared worlds, `landing.face` a declared face of the copy's type.
- wire: `/_schema` worlds gain `messages` + `on_absent`, faces gain `messages`;
`_copies[]` gains `onSuccess`.
- SPA: EntityDetail, EntityList, KanbanView, DynamicForm, WorldBadge, copy
landing.
- docs: content-states guide Step 6, data-entry guide worlds section,
metamodel reference tables.

Address-grammar hardening moved to its own ticket.
