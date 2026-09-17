---
id: RR-SG15U4
type: review-response
title: The CSS comment claimed DOM parity it does not have, and a shared rule is dead here
finding: 'The task-list comment in milkdownEditor.css said the node view "emits the same shape marked and goldmark produce", then conceded four lines later that it adds a wrapper element - the contradiction pattern commentlint flags as duplication. More usefully, the reviewer pointed out a consequence the comment did not: the shared sheet''s LOOSE-item rule (`li > p:first-child:has(> input…)`) can never match the editor, because the input sits outside the `<p>`. That is precisely why the local paragraph-margin reset is needed.'
severity: minor
resolution: Rewritten to say the node view matches the TIGHT shape the sheet looks for (an input as the li's first child), which is why those rules apply, while noting it is not byte-identical because ProseMirror requires a contentDOM wrapper. The dead loose-item rule is now stated explicitly and connected to the margin reset below it.
status: addressed
---
