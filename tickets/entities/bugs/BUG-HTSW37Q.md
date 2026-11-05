---
id: BUG-HTSW37Q
type: bug
title: Conflict routes answer 500 under a row-dependent read grant on fsstore
description: On fsstore a conflicted file stays indexed but fails to load. Under a row-dependent read grant the row gate surfaces that load error, so the conflict routes and the type's lists answer 500. It fails closed.
priority: low
effort: m
why1: 'The row gate for a when: grant runs a graph query over the type, and the naive graph query returned the load error of the conflicted file.'
why2: fsstore keeps a conflicted file in its index, but loading it fails with errConflictedFile, and collectByType aborts on any row error.
why3: The store contract had no name for a row that exists but cannot be read, so a query could not tell it from a broken store and failed the whole query.
why4: The memory store used by the handler tests never has a conflicted row so no test ran a row-dependent grant against one.
why5: Store backends differ in which rows can be unreadable and the graph-query contract did not say what a query does with such a row.
prevention: GraphQuery.SkipUnreadable states the contract. Only the ACL read-grant query sets it so a left-out row is hidden. Endpoint matching and every other query fail closed. TestGraphQuery_SkipUnreadable runs it on fsstore and conferredfaces_test pins that the ACL sets the flag.
started: "2026-11-05"
completed: "2026-11-05"
status: done
---

## Description

On the file store a conflicted entity file stays in the index but its load
fails. A row-dependent read grant (when:) makes the row gate list the type\, and
that load error comes back. The conflict list\, detail and resolve then answer
500 for that principal\, and so does every list of that type. It fails closed:
nothing is disclosed.

## Fix direction

Decide what the gate judges for a row the store cannot load. Options: judge it
as hidden with a clean 404\, or judge the last committed version of the file.
Pin it with an fsstore test.

Found in review of BUG-Q3Z15V.
