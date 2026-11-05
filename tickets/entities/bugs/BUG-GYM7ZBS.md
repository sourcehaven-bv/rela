---
id: BUG-GYM7ZBS
type: bug
title: A body line starting with a conflict marker makes the file unreadable
description: On fsstore a body line that starts with <<<<<<< makes the written file read as conflicted, so the row fails every read and is hidden from role_relations readers.
priority: medium
effort: s
status: backlog
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
