---
id: TKT-7XS6AC
type: ticket
title: Import existing .rela/comments/ YAML when adopting a database backend
kind: enhancement
priority: medium
effort: m
status: backlog
---

## Description

Deferred from TKT-OGTVJW as RR-PAF29J.

An operator who commented on the filesystem build and then switches to postgres
or sqlite finds their comments gone. The YAML files are intact under
`.rela/comments/`, but nothing reads them and nothing warns. Shipping a second
flavour of "your comments are invisible" is a pointed thing to do in the arc
that exists to fix the first one.

## Why it was deferred rather than done

TKT-OGTVJW's scope said so explicitly: "No migration of existing
`.rela/comments/` YAML into a database — an operator adopting postgres starts
with an empty comment store, as they do for every other table."

That is defensible. Entities, relations, attachments and search all start empty
on a backend switch, so comments are not being singled out. The user-facing
guide now states plainly where each backend keeps comments, so the behaviour is
discoverable before an operator commits to the switch.

What makes it worth a ticket anyway: every one of those other tables has a
documented adoption path, and comments now have the machinery for one. The
`comments.Store` interface is backend-agnostic, `filecomments` can read the old
directory, and the target backends are already wired.

## Approach sketch

A one-shot import in `internal/datamigration`, alongside the other adoption
paths, rather than anything automatic on open:

1. Read through `filecomments` over the project's `.rela/comments/`.
2. Write through the configured backend's `comments.Store`.
3. Refuse, or require a flag, when the destination is non-empty — the same
shape `perfseed` uses.
4. Operator-invoked and audited, per CLAUDE.md's rules for the sanctioned
raw-store exceptions.

Import is naturally idempotent on `(target_key, id)`: a comment id is minted per
comment, so re-running should skip what is already there rather than duplicate
it. Worth reusing `comments.MergeThreads`' "destination wins" rule so the answer
matches what Rename already does.

## Out of scope

No automatic migration on first open. A silent bulk write into a database the
operator just pointed at is the wrong default, and an empty comment store is a
recoverable state where a half-finished import is not.

## Acceptance criteria

1. An operator can import an existing `.rela/comments/` tree into the postgres
or sqlite backend with one explicit command.
2. Re-running the import does not duplicate comments.
3. The import refuses a non-empty destination unless explicitly forced.
4. Every imported comment keeps its original id, author and timestamp — an
import must not re-attribute or re-date commentary.
5. Faced threads (`id@face`) import to the same keys they had on disk.
