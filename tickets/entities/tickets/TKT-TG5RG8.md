---
id: TKT-TG5RG8
type: ticket
title: Document the help endpoint as public by design
kind: docs
priority: low
effort: xs
status: done
---

## Description

`/api/help/{entityType}` performs no read-authorization check on the entity
type: any principal reaching the endpoint gets the help text for any type.

GitHub issue #1176 proposes gating it on read authorization.

## Decision: document as public by design, do not gate

Decided by the project owner. **Help is public, like the schema itself.**

The clarifying test is an open-source deployment: the entity model, the field
descriptions and the help text are all in the repository. Guarding the endpoint
that serves them protects nothing an interested party cannot read on GitHub — it
only makes the running app less usable than its own source.

That is the distinction the issue misses. Help text is documentation ABOUT the
application, not data IN it. Read authorization governs the latter.

## Supporting evidence in the code

`/api/v1/_schema` (`internal/dataentry/api_v1.go:84`, `handleV1Schema` :1158)
serves the complete entity/relation/custom-type model — every type name, every
property, every relation — with NO read gate. It is registered in the same
router as the help endpoint.

So gating help would be inconsistent rather than protective: the same
information is already served, ungated, one endpoint over. Closing this by
adding a gate to help alone would produce a boundary that looks deliberate while
defending nothing.

Note also CLAUDE.md's rule that a settled decision should read as settled rather
than open. The current code neither gates nor explains, which is what left room
for this finding.

## Scope

IN: document, in the help handler's godoc and in the user-facing docs, that help
is public by design and why — including the `_schema` parallel, so the next
reviewer sees the reasoning rather than re-deriving the finding.

OUT: any authorization change to `/api/help/`.

OUT: changing `_schema`'s exposure. It is out of scope here, and the argument
above depends on it staying as it is — if that ever changes, this decision
should be revisited rather than silently inherited.

## Acceptance

A reader arriving at `handleEntityHelp`, or the next IB review, finds the
decision and its reasoning. It must answer: why is this ungated, what else is
already public, and what would have to change for that to be wrong.

`docs/` is GENERATED from `docs-project/` entities; edit the source.
