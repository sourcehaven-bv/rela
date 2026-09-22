---
id: RR-Z5WLG1
type: review-response
title: Load path preserves pre-existing hostile URLs; decision was unstated
finding: 'The plan argued write-side validation is needed because the mark serializes to the markdown file, but did not follow that argument through to the load path. A body already containing a javascript: URL is parsed into the mark unvalidated; if the user edits anything else, guardWriteBack returns ''edited'' and the editor rewrites the body with the hostile URL preserved. A reader would conclude the plan closed the XSS surface when it closed only the insert half.'
severity: critical
resolution: 'Not sanitizing on load is the correct trade (rewriting would break the losslessness contract that rawHtmlPassthrough.test.ts:109 protects, and the URL is already defanged at every render sink). The decision and its rationale are now stated explicitly in the Security section, with AC 12 pinning that a pre-existing javascript: link round-trips unchanged. Cleaning existing bodies is a migration, not an editor behaviour.'
status: addressed
---

**Finding (design review of PLAN-RY25IP).** The plan's own security argument is
symmetric and it did not follow it through. It says write-side validation is
required because "the original string stays in the ProseMirror mark and still
serializes into the markdown file", and that render-time sanitizing is
insufficient. The same reasoning applies on the way in:

1. A body containing `[x](javascript:alert(1))` is parsed by
`linkSchema.parseMarkdown` with no sanitization; the raw string is in the mark.
2. Render-time `toDOM` blanks it, so nothing executes. Fine.
3. The user edits an unrelated paragraph, `dirty` becomes true, `guardWriteBack`
returns `edited`, and the editor writes the body back out — hostile URL
included, because `toMarkdown` serializes `mark.attrs.href` raw.

So the editor preserves and re-persists a URL it would never accept. The ticket
says "the editor should not be the component that admits one"; under the
original plan it was still the component that carried one.

**Resolution — the behaviour stays, the silence does not.** Sanitizing on load
would be wrong: `rawHtmlPassthrough.test.ts:109-120` deliberately pins that the
editor is lossless, calling parse-time sanitizing "a data-integrity bug wearing
a security fix's clothes", and that test is right. The URL is inert at every
render sink (preset `toDOM` in the editor, DOMPurify in the rendered view).

What was missing was the decision being written down. The Security section now
states it, with the rationale and the scope limit ("this ticket closes the
insert half of the surface, not the whole surface"), and AC 12 pins the
round-trip so a later reader cannot mistake it for an oversight and "fix" it
into a data-integrity bug. Cleaning existing bodies is a migration —
operator-run and auditable — not an editor behaviour.
