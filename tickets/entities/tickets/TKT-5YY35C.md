---
id: TKT-5YY35C
type: ticket
title: Split self-contained features out of dataentry into subpackages
kind: refactor
priority: medium
effort: l
status: ready
description: Move caldav, gantt, webhooks, customassets and customapps into internal/dataentry subpackages behind access; dataentry stays the composition root.
---

## Description

Move the self-contained features out of `internal/dataentry` into subpackages.
dataentry stays the composition root and mounts their routes. Modelled in
archplan as suggestion sg-6b833e13 (thread th-88b9cf7d, accepted).

## Scope

Two PRs, after TKT-WFS778 lands so the subpackages use `internal/access` instead
of acl/visibility:

1. `internal/dataentry/caldav` (caldav*.go, about 3,000 lines).
2. `internal/dataentry/gantt`, `webhooks`, `customassets` (/_custom/),
`customapps` (/_apps/).

Each subpackage may import only what its files import today, minus acl and
visibility. The archplan target pins the allowed edges; arch-lint gets the same
rules.

Out of scope: documents, export, comments, history and commands (more entangled
with `App`; revisit after access). No behaviour change.

## Acceptance

- Packages exist with only the allowed imports; `just arch-lint` enforces them.
- `App`'s plimsoll method count goes down; the directive is ratcheted.
- All existing tests pass unchanged in behaviour; e2e for CalDAV and gantt pass.
