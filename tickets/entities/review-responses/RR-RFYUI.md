---
id: RR-RFYUI
type: review-response
title: Rewriter idempotency untested
finding: 'Today''s rewriter is idempotent by accident: non-form is no-op, form strips pre-existing return_to. Once every internal link grows return_to, any double-rewrite path (watcher refresh, preview, debug dump) must converge. Plan''s test list doesn''t cover double-rewrite. Add: (1) RewriteDocumentLinks(rewritten, path, nil) byte-equal to one-pass; (2) rewriting with returnPath=''/A'' then ''/B'' yields the ''/B'' variant, not both.'
severity: significant
resolution: 'Added AC10 with two test cases: (a) double-rewrite with same returnPath is byte-equal to single-rewrite; (b) rewrite with /A then /B yields only /B.'
status: addressed
---
