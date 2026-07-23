---
id: TKT-YLFJRG
type: ticket
title: Ship a generated operator handbook for the demo tracker (dogfood rela-docs)
kind: docs
priority: medium
effort: m
status: done
---

## Problem

The rela-docs generator (FEAT-G4VO53) has a doc language, resolvers, and
screenshots, but no committed end-to-end artifact that demonstrates them. The
README isn't a good fit — its dynamic content is *instance data* (rela's real
guide entities read from disk), and a manual seeds a fresh **memstore** by
design (schema from the real project; entities only from the manual's own
`create`/`link`). So a manual can't reproduce the README's live doc tables.

Manuals shine at **schema-derived** content (types, lifecycles, ACL roles) plus
**seeded** illustrative examples — exactly an operator handbook for a project.

## Change

Author and commit `docs/examples/ticket-tracker-manual.md` (source:
`README`-style `.rela` manual) — a proper operator handbook for the demo ticket
tracker (`prototypes/data-entry/project`), built by `rela-docs build`:

- Intro prose (hand-written) + `description()` (the demo project's own).
- `typeref{ticket}`, `lifecycle{ticket,status}` (mermaid with transition
labels), `roles_matrix{ticket}` (editor/viewer from the showcase `acl.yaml`).
- Seeded example tickets via `create`/`link`, then `entity{}`/`graph{}` and a
`screenshot{}` of the seeded ticket form with annotations.

Add a `just` target to build it; commit the rendered `.md` + screenshot PNG.
Link it from the README as a live example of the doc generator.

## Dependency

Depends on the rela-docs binary (TKT-X00CDI / PR #1187). The build step needs
the frontend + Chrome for the screenshot; committed artifacts are the source of
truth until #1187 merges.

## Acceptance criteria

- `docs/examples/ticket-tracker-manual.md.rela` builds via `rela-docs build --project prototypes/data-entry/project` to a committed handbook.
- Renders a real field table, mermaid lifecycle with transition labels, editor/viewer roles matrix, and an annotated ticket-form screenshot.
- README links to it; markdown lint passes.

Implements FEAT-G4VO53 (capstone dogfooding).
