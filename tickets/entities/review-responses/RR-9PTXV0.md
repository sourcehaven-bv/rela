---
id: RR-9PTXV0
type: review-response
title: _pendingValue never cleared after mount; toolbar/theme forked without drift guard
finding: (1) _pendingValue held a duplicate of the document for the editor's whole lifetime after mount (invariant 'only meaningful while _editor==null' was undocumented/unenforced). (2) The toolbar config and theme CSS were hand-forked from MarkdownEditor.vue with no drift guard — the exact failure mode the tokens.css drift test exists to prevent, applied inconsistently.
severity: significant
resolution: (1) _pendingValue cleared at end of _mount with the invariant documented. (2) Added reciprocal DRIFT cross-reference comments in MarkdownEditor.vue, relaEditorTheme.css, and relaEditor.ts; full extraction tracked as TKT-D2JML7 (shared plain-TS core).
status: addressed
---
