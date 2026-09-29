---
id: BUG-4SYAA6
type: bug
title: History, restore and history-purge ignore faces
description: HTTP restore and CLI history/restore/purge act on the zero face, so faced entities have no usable history and cannot be erased.
priority: high
effort: m
why1: The history, restore and purge surfaces took a bare entity id and called the zero-face readers (ListVersions, GetVersion, a purge request with no face), so on a faced type they read or erased the lineage of a row that does not exist.
why2: Versioning became per face (migration 0012 keyed entity_versions by id and face) but the callers kept the pre-faces short-form API, which silently means the zero face; nothing refused a bare id on a faced type.
why3: The short-form wrappers are valid calls on every type, so the compiler and the tests (seeded on unfaced types) accepted them. The restore-recreate path also minted a new id via CreateEntity while reporting the old one, and no test compared the two.
why4: The faces rollout changed the storage key before the read and write surfaces took an address, and the zero-face reads were not pinned by the archguard allowlist until TKT-2528AB added it.
why5: A default argument (the zero face) that is valid for unfaced types hides a missing address on faced ones. The systemic gap is an API whose easiest call is the wrong one for half the types.
prevention: Every history surface resolves an ID@face address (the resolver on HTTP, historyAddress in the CLI) and the CLI refuses a bare id on a faced type, naming the faces. The zero-face archguard allowlist lost the history entries and may only shrink. storetest.RunVersionTests pins that snapshots record their face and that entity and relation purges stay inside one face or tail. The e2e faces-history spec runs on postgres.
status: done
---

## Problem

Reported by the face-awareness inventory, not yet verified:

- HTTP history restore is face-blind (`history_handler.go:138`, `history_restore.go:39,57`).
- CLI `history`, `restore` and `history-purge` pass the zero face (`history.go:42`, `restore.go:39`, `history_purge.go:52-59,124-127`).
- The short-form wrappers `ListVersions`, `GetVersion` and the purge requests hard-code face `""`.

The purge gap blocks compliance erasure of a faced entity's history.

## Expected

Every history, restore and purge surface takes an `ID@face` address and acts on
that face's lineage.
