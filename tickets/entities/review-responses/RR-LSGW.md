---
id: RR-LSGW
type: review-response
title: Add `satisfies EasyMDE.Options` to make autoDownloadFontAwesome rename a typecheck error
finding: 'If EasyMDE renames or removes the autoDownloadFontAwesome flag in a future release, our option becomes inert silently. EasyMDE''s heuristic fallback (search styleSheets for the maxcdn URL) would catch this at runtime only via the e2e test. Belt-and-braces: add `satisfies EasyMDE.Options` to the options literal so a rename fails at typecheck.'
severity: minor
resolution: Added `satisfies EasyMDE.Options` to the options object (MarkdownEditor.vue:58-77). If EasyMDE renames or removes autoDownloadFontAwesome in a future version, `npm run typecheck` fails at the satisfies clause before any runtime regression can occur.
status: addressed
---
