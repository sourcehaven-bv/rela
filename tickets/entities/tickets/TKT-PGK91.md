---
id: TKT-PGK91
type: ticket
title: Detect git-crypt encrypted files at fsstore and show inaccessible placeholders in data-entry
kind: enhancement
priority: medium
effort: m
status: review
---

## Problem

Some users use [git-crypt](https://github.com/AGWA/git-crypt) to keep parts of
their rela project (entities and relations) encrypted at rest in git. When a
collaborator clones the repo without setting up the git-crypt key (or runs
without `git-crypt unlock`), every encrypted file on disk is the raw ciphertext:
a binary blob starting with the magic header `\0GITCRYPT\0`.

Today rela has no awareness of this. Symptoms:

- The markdown parser fails (or worse, succeeds with garbage) on the ciphertext.
- The data-entry UI shows the entity as broken/missing, displays binary data, or 500s.
- The user has no clue that the cause is "this file is git-crypt encrypted, you need to unlock".

## Goal

Detect git-crypt encrypted entity/relation files at the storage boundary
(fsstore) and surface them to the data-entry UI as a first-class `inaccessible
(encrypted)` state. The UI shows a placeholder with a clear message and a help
link pointing the user at `git-crypt unlock`, so misconfigured collaborators
understand and can fix it.

## Scope

In scope:

- Detect the git-crypt magic header (first 9 bytes `\0GITCRYPT\0`) when fsstore reads an entity or relation file.
- Treat such files as a typed "inaccessible" load result (encrypted, key not present) instead of a parse error.
- Expose the state to data-entry so list views and detail views render an inaccessible placeholder with actionable text instead of the ciphertext / parse error.
- Read-only behavior: editing/saving an inaccessible entity is blocked with a clear error.

Out of scope:

- Generic "unparseable file" handling — this ticket is git-crypt-specific. Other parse errors keep current behavior.
- Anything in the CLI / MCP / tracer surface beyond what is needed to keep them functional. Failing loudly there is acceptable; the user-visible improvement is data-entry only. (Detection can still live at fsstore so consumers may opt in later.)
- Decryption. We never try to read git-crypt keys. We only detect the header.
- git-crypt status of `metamodel.yaml`, `templates/`, `groups.yaml` etc. — those stay cleartext per existing project conventions.

## Acceptance criteria

1. When fsstore reads a file under `entities/**/*.md` or `relations/**.md` whose first 9 bytes are `\0GITCRYPT\0`, the load surfaces an `inaccessible` outcome (typed, distinguishable from generic parse errors).
2. The data-entry SPA list view renders such entities/relations with a clear "🔒 inaccessible (encrypted)" indicator (placeholder row), preserving the entity ID where it can be derived from the filename.
3. The data-entry detail view for an inaccessible entity renders a placeholder page with: short explanation, the filesystem path, and a help link explaining how to unlock with git-crypt.
4. Attempting to edit/PATCH an inaccessible entity returns a structured error (HTTP 4xx) and the SPA shows a clear "cannot edit, file is encrypted" message.
5. Unit tests cover header detection (positive + negative cases including 8-byte files, files starting with `\0GITCRYP` only, unicode/binary content not matching the header).
6. Integration tests cover the data-entry list and detail rendering paths against a fixture project containing one git-crypt encrypted entity and one relation.
