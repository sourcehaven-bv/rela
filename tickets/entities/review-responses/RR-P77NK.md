---
id: RR-P77NK
type: review-response
title: Default anchor format does not match common Markdown→HTML/PDF anchor algorithms
finding: Plan defaults entity_refs to '[Title](#<id-lowercased>)'. But Pandoc, goldmark auto-heading-id, and markdown-it-anchor all derive anchors from the heading TEXT (lowercased + non-alnum to '-'), not from any external ID. So the default link will 404 unless the user emits explicit ID-based anchors. Either change the default to title-slug or document the assumption clearly.
severity: minor
resolution: 'Default changed to title-slug (Pandoc-style: lowercase, non-alnum runs → ''-'', trim). New opts.style supports ''title-slug'' (default) and ''id'' (lowercased ID anchor). opts.format still wins when provided. AC9 verifies default; AC10 verifies style="id"; AC12 verifies format precedence.'
status: addressed
---

# Finding

`entity_refs` defaults to `[Title](#<id-lowercased>)`. The plan says this
"matches common Markdown→PDF anchor conventions." It does not:

- Pandoc derives section anchors from heading **text** (lowercase + run of
non-alnum → `-` + trim).
- Goldmark's `auto-heading-id` extension does the same.
- markdown-it-anchor ditto.

If the user composes their doc by emitting `# {entity.title}` per included
entity, the auto-generated anchor is from the title, not the ID. The default
link will 404.

The default works only if the user *also* emits ID-based anchors (e.g. `<a
id="tkt-1"></a>` or Pandoc-style `# Title {#tkt-1}`).

# Resolution

Change the default to derive the anchor from the title:

> Default `format`: lowercase the title, replace runs of non-`[A-Za-z0-9]`
> with `-`, trim leading/trailing `-`. Yields `[Title](#title-slug)`.

Document the slug rule precisely (small surface area). Provide a copy-pasteable
`format` snippet for users who emit ID-based anchors.

Update AC8 to assert the new default.
