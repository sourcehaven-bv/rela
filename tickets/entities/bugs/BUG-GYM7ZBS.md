---
id: BUG-GYM7ZBS
type: bug
title: A body line starting with a conflict marker makes the file unreadable
description: On fsstore a body line that starts with <<<<<<< makes the written file read as conflicted, so the row fails every read and is hidden from role_relations readers.
priority: medium
effort: s
why1: fsstore's parseDocument refused any file with a line starting with the start marker, so a body line such as one documenting the marker made the row unreadable.
why2: The detector checked only for the start marker at column 0, not for a complete block of start, separator and end markers in order.
why3: BUG-WN6D narrowed a substring match to a line-anchored one but kept the one-marker test; two copies of the detector (markdown and fsstore) drifted apart from conflict.FindMarkers, which already required the full block.
why4: No store conformance test wrote a body that documents conflict markers and read it back.
why5: Format rules such as conflict detection were copied per package instead of owned by one function with one test table.
prevention: One scanner (markdown.ConflictBlocks) serves fsstore, the conflict list and the resolver. It requires a complete block, fenced or not, because a merge moves shared fence lines out of a conflict. storetest RoundTripsBodyDocumentingConflictMarkers pins every backend; formatDocumentOrdered refuses a document that would read as conflicted.
started: "2026-11-05"
completed: "2026-11-05"
status: done
---

## Description

On the file store a normal write can produce a file that reads as having merge
conflicts. `formatDocumentOrdered` writes the body as given, and
`hasLineAnchoredConflict` treats any body line that starts with `<<<<<<<` as a
conflict marker. BUG-TOXQAA fixed this for property names only.

A writer who may edit an entity's body can therefore make its file unreadable.
The row then fails every store read until someone edits the file. Since
BUG-HTSW37Q, it is also hidden from a principal whose read grant comes through
`role_relations`, and it appears in the conflict list.

Found by the security review of BUG-197XPAU and BUG-HTSW37Q.

## Fix direction

Do not let a write emit a column-0 conflict marker in the body. Either escape
such a line when writing, or refuse the body with a 422. Pin it with an fsstore
test that writes such a body and reads the row back.
