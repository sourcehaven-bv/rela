---
id: RR-PXLIST
type: review-response
title: The list/export path is named as scope but its different threat model is not analysed
finding: 'The rescope correctly adds ExecuteListDocument as a third entry point, but stops at "verify
  each" and "check it explicitly". That is a TODO, not analysis, and the list path is the one where the
  ticket''s reasoning is weakest.


  What is actually different: RenderListMarkdown (document.go:316) serves `lists.<id>.export_render`,
  reached from export_list.go:145 — a download endpoint, not a document panel. So the framing that carried
  the whole ticket ("a GET that renders a page must not mutate") needs re-deriving for a path whose output
  is a file the user downloads.


  Two concrete questions the ticket should answer before implementation, not during:


  1. Is the export endpoint a GET? If it is, the same idempotence argument holds and the change is uniform.
  If it is a POST, the argument for removing writes there is materially weaker and the ticket should say
  whether it still applies.

  2. Does export_render have a legitimate reason to write? An export that stamps "last exported at" on
  the entity, or records an export-audit entity, is the single most plausible legitimate write during
  a render in the whole codebase — and it is exactly the pattern this change would break.


  Deciding this during implementation means deciding it under pressure to make the diff uniform. It is
  a design question.'
severity: significant
status: open
---
