---
id: RR-T0MGE
type: review-response
title: Rewriter tests use literal & where goldmark emits &amp;
finding: internal/dataentry/document_test.go:178-180 feeds ?prop.status=draft&rel.implements=FEAT-001 directly to RewriteDocumentLinks. In production the rewriter's input is goldmark HTML, which encodes & as &amp; in href values — the test never exercises that input shape. Add a subtest with &amp; in the existing query to catch ampersand-handling regressions.
severity: significant
resolution: Added a 'goldmark-encoded ampersand in query preserved' subtest feeding href='?a=draft&amp;b=FEAT' to RewriteDocumentLinks. The new stripQueryKey helper (RR-CKLD2) splits on both & and &amp; so rewriter output round-trips goldmark's encoding untouched.
status: addressed
---
