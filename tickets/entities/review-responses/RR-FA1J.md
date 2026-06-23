---
id: RR-FA1J
type: review-response
title: contentRef looks unused -- flag for follow-up
finding: |
  Reading useAutoSave.ts end-to-end, opts.contentRef is never read. lastSeenContent is a local. mergeServerResponse calls opts.applyServerContent. The form's content ref is observable elsewhere. Suspect leftover; out of scope for this ticket.
severity: nit
status: deferred
reason: |
  Removing contentRef from AutoSaveOptions would touch DynamicForm and is genuinely out of scope for IHC7A. Flagged for a future cleanup ticket. EntityDetail's content-only instance pays the small cost of fabricating a contentRef (a one-line computed) -- acceptable. Documented as part of RR-FA1C's resolution: AutoSaveOptions.contentRef gets a jsdoc note clarifying it's read-only-shape.
---
