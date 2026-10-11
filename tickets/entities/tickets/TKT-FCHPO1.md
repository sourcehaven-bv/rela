---
id: TKT-FCHPO1
type: ticket
title: Comments name capabilities, not vendor libraries or backends
kind: chore
priority: low
effort: m
started: "2026-10-10"
completed: "2026-10-11"
status: done
description: Reword comments that name a vendor library or storage backend the package does not depend on.
---

## Description

The archplan glossary lists 77 places where a component names a vendor library
or storage backend it does not depend on: goldmark, pgx, sqlite, bleve,
fsnotify, jwt, neoq and others. 63 are in comments only. Such comments go stale
when the vendor or backend changes.

Rule: comments and identifiers describe what the package does, in its own terms
and those of its direct dependencies. Name the capability, not the vendor ("the
markdown renderer", "a transient store error", "a backend that implements
`BulkMigrator`").

Origin: archplan threads th-58000adf, th-a273f2ce, th-e28e0606, th-b05ddb9e.

## Scope

In scope:

- Reword the comment-only matches. Includes acl/storegraph.go:31,
store/store.go:747-755, store/worldresolve.go:96 and
predicatefns/predicatefns.go:79-81.
- Leave a match when the package wraps or implements that vendor or backend
(for example sqlite in sqlitestore or sqlitecomments). Those are false
positives.

- Rename `store.FaceSelection.Lowered` to `FlattenedWhenUnranked`
(14 references; "lowered" is compiler jargon). Thread th-fd62b77f.

Out of scope:

- Identifier renames (14 components). They change APIs and are listed for a
follow-up. `RejectEffective(jwtWired)` is renamed in TKT-WFS778.
- No behaviour change.
