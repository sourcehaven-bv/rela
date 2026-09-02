---
id: TKT-2P9S72
type: ticket
title: 'analyze: flag relation files whose filename disagrees with their content'
kind: enhancement
priority: medium
effort: s
status: done
---

## Description

`fsstore` keys relations **entirely on the filename** — `syncRelations` parses
`FROM--TYPE--TO.md` and never reads the file to check the parsed triple against
the `from`/`relation`/`to` in the frontmatter. A file whose name and content
disagree is therefore indexed under the *filename* triple, and the relation its
content describes simply does not exist as far as the graph is concerned.

Reported from a real project (GitHub issue #1004): eight relation files like

```
filename:  PRS-FLOW-1HL6--wordtUitgevoerdDoor--PRS-FUNC-8Q7E.md
content:   to: PRS-FUNC-0Q7E        # correct ID, differs from the filename
```

produced `PRS-FUNC-0Q7E must have at least 1 ... relation(s), has 0`, with no
indication of why. The entity named in the filename did not exist at all.

## What is already fixed

The rename path that likely caused it. `fsstore.renameEntity` rewrites each
incident relation under its **new** filename and removes the old one — verified:
after renaming `REQ-1` to `REQ-999`, the only file present is
`SOL-1--implements--REQ-999.md`. So new corruption of this shape is not being
produced.

## What is still missing

The issue's second suggestion, and the reason the reporter lost time: nothing
detects a mismatch that already exists. A project carrying legacy corruption
gets a cardinality error pointing at the *victim* entity, with no path back to
the malformed file.

`rela analyze` should report it directly: for each relation file, compare the
filename triple against the frontmatter triple and flag disagreement.

## Out of scope

Keying relations on content instead of filename (the issue's third suggestion).
That is a storage-model change with migration implications; a detector is the
cheap, non-destructive half and is what turns a mystifying symptom into a named
finding.
