---
id: BUG-HTSW37Q
type: bug
title: Conflict routes answer 500 under a row-dependent read grant on fsstore
description: On fsstore a conflicted file stays indexed but fails to load. Under a row-dependent read grant the row gate surfaces that load error, so the conflict routes and the type's lists answer 500. It fails closed.
priority: low
effort: m
status: backlog
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
