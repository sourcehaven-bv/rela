---
id: TKT-5WSCEZ
type: ticket
title: Config linting and cross-file impact analysis over the projected schema graph
kind: enhancement
priority: medium
effort: m
status: backlog
---

## Problem

First user-visible feature of schema-as-graph, and the natural first milestone:
wire the projector into a read path so rela can analyze **its own
configuration**.

Needs no write path and no storage decision — it stands alone and justifies the
projector work even if everything after it stalls.

## What it buys

**Dead-config detection.** The spike found `doc_kind` (TKT-85Q6U5) — a custom
type defined and never referenced. Real defect, currently undetectable.

**Cross-file impact analysis.** `trace to ticket-status` answers *"what breaks
if I change `ticket.status`?"* → 3 UI surfaces (`all_tickets` column,
`create_ticket` field, `edit_ticket` field), the `doc-task` type sharing its
custom type, and ~15 validations. **Neither `schema.yaml` nor `data-entry.yaml`
can answer this today**, because the edges cross files.

**Dangling-reference checks.** A form field binding a nonexistent property; a
list column traversing an undeclared relation; a `styles:` block matching no
custom type. The spike found zero of these in `tickets/` — the config is
internally consistent — but the check has no cost once the projection exists.

## Scope

- A read path exposing the projection to `analyze` and `trace`
- Orphan / cardinality / dangling-reference checks over config
- Report the *findings*, not the projection: users should see "custom type `doc_kind` is unused", not entity IDs

## Open questions (resolve when work starts)

- **Surface: a new `rela analyze config` subcommand, a flag on the existing analyze commands, or a separate binary?** The projected graph is a different subject from the project's data, so overloading `analyze` may confuse more than it reuses.
- **Does this reuse `internal/analysis` directly** (which takes a `store.Store`), implying the projection must be presented as a store — or does it get a purpose-built checker over the in-memory projection? The former reuses more; the latter avoids pulling a store abstraction in early.
- **How are artifacts suppressed?** §5.7 is the cautionary tale: a projection gap shows up as a confident-looking lint finding. Findings should probably be gated on the conformance test passing.
- **Does it run in CI over `tickets/` and `docs-project/`** as a dogfood check?

## Context

Findings `.ignored/schemaspike/FINDINGS.md` §5.1 (real finding), §5.7 (artifact
ratio). Depends on the projector and the fidelity work.
