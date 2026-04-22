---
id: RR-O0PVX
type: review-response
title: Stale comment in rewriter references legacy edit forms
finding: internal/dataentry/document.go:429-430 comment says "when returnPath is empty and the link isn't a legacy edit form, there's nothing useful to inject" but the code below no longer special-cases edit forms. Trim to "when returnPath is empty, there's nothing to inject; pass through."
severity: nit
resolution: 'Trimmed the stale comment to: ''When returnPath is empty, there''s nothing useful to inject; leave the query and fragment alone.'' No more phantom reference to legacy edit forms.'
status: addressed
---
