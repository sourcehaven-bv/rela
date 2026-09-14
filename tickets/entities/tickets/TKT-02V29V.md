---
id: TKT-02V29V
type: ticket
title: Webhook append_section flattens interpolated values rather than the operator's template
kind: enhancement
priority: medium
effort: s
tags: security
status: done
---

## Description

`append_section` flattened the *finished* interpolation, so it could not tell a
newline the operator typed in `content:` from one the producer sent in the
payload. Both were collapsed to spaces.

That over-reaches. The stated threat was only ever the payload planting its own
markdown structure: a `## Heading` arriving inside `{{body.output}}` becomes a
SIBLING of the section being appended to, so every later delivery targeting that
section lands above it and the document silently reshapes itself. An operator
writing a multi-line template is not that threat — they are describing the shape
of their own document.

The cost was that a structured timeline entry, a heading with detail beneath it,
could not be expressed at all. That is the natural form for an incident
timeline, which is the motivating use case for the whole webhook pipeline.

## Change

Move `flattenToLine` off the finished string and into `interpolate()`, applied
per substituted value. That is the seam every producer-controlled string passes
through, and the only one that can distinguish the two sources.

## Scope

In scope: the flattening site, its godoc, a test pinning the operator half, and
the `docs/webhooks.md` section explaining the asymmetry.

Not in scope: the header allowlist, the retry budget, or any change to
`markdown.AppendToSection`, which stays a general utility whose other callers
may legitimately append multi-line markdown.

## Acceptance criteria

1. An operator's multi-line `content:` reaches the document with its line
structure intact.
2. A newline inside an interpolated VALUE is still flattened to a space, whether
the template is one line or many.
3. The existing injection test is unchanged and still passes: a payload cannot
plant a sibling heading.
4. Flattened content is never dropped. Refusing would discard an alert whose
producer will not resend it.

## Security note

The asymmetry is the point, and it is directional. Operator-authored newlines
are trusted; producer-supplied ones are not. If the flattening ever moves back
to the finished string, criterion 1 breaks; if it moves off the value, criterion
2 and 3 break and injected data can forge section structure. Both directions are
pinned by tests.
