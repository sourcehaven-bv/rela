---
id: RR-ICZAM2T
type: review-response
title: Editing outside_edits changes the flow version
finding: outside_edits is part of the hashed declaration, so editing a rule bumps the flow version.
severity: nit
reason: The version identifies the declaration as written; excluding one key from the hash makes the version lie about the source it names.
status: wont-fix
---
