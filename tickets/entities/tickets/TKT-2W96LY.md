---
id: TKT-2W96LY
type: ticket
title: Entity IDs must start with a letter or digit
kind: refactor
priority: low
effort: xs
status: done
---

## Description

`entity.ValidateID` bans a leading `-` but still accepts a leading `_`, so
`_private` is a legal entity ID today. Both are malformed as identifiers: `-rf`
reads as an option flag, `_foo` as a hidden/private marker.

Tighten the grammar so an ID must **start with a letter or digit**:

before: ^[A-Za-z0-9_-]+$   plus an explicit leading-dash check after:
^[A-Za-z0-9][A-Za-z0-9_-]*$

### Why — and why NOT as a security control

The existing leading-dash check was introduced (TKT-IZGF7T) as defence in depth
against argument injection, because the entity ID reached an `sh -c` string in
the documents renderer.

That framing is being retired: TKT-QGHNVA removes request-derived data from the
command line entirely, so command safety is structural rather than filtered.
This ticket therefore **re-justifies** the rule on well-formedness grounds and
says so explicitly in the godoc — an identifier opens with a letter or digit,
the same as DNS labels, Kubernetes resource names, and most language
identifiers.

Keeping the rule but relabelling it matters: a future reader must not treat it
as the thing preventing argument injection, or they will build on a guarantee
that no longer lives here.

### Scope

IN: the grammar and its rationale, plus test coverage for a leading underscore
(currently untested because it was legal).

NOT IN: removing `{id}` from the shell string (TKT-QGHNVA) — that is the
structural fix this rule stops standing in for.

### Back-compat

None affected. Every entity id across `tickets/` and `docs-project/` was
scanned: zero start with a non-alphanumeric character. Generated ids are prefix
+ digits; manual ids are lowercase slugs.

### Acceptance criteria

1. `_private` and `-rf` are both rejected, naming the reason
("must start with a letter or digit").
2. `TKT-001`, `ai-integration`, `a`, `A1` still valid.
3. storetest conformance and the ValidateID fuzz oracle stay green in every
backend (the oracle is bidirectional, so a stricter rule requires the stores to
actually reject the newly-invalid ids).
4. The godoc states this is well-formedness, not command safety.
