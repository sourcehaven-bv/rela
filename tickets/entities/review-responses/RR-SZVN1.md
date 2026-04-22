---
id: RR-SZVN1
type: review-response
title: fakeScriptCall uses exported fields in a test-only package
finding: internal/dataentry/document_script_test.go:68-73. Struct is private to the test file. Lowercase the fields for style consistency with the rest of the test code.
severity: nit
resolution: fakeScriptCall fields lowercased (path, documentID, entryID, timeout). Consistent with private-struct-in-test-file style.
status: addressed
---

From post-impl cranky review. Trivial style fix.
