---
id: RR-DOCWIRE
type: review-response
title: TestElevatedDeps_GrantsBypassBinding bypasses the production constructor it claims to verify
finding: 'The test comment (elevated_document_test.go:256-263) claims it proves "the elevated reader actually
  arrives" because other tests stub the renderer.


  It builds lua.NewWriter(deps, &out) DIRECTLY. Production does not: it goes ExecuteStandaloneDocument
  -> runDocumentScript -> NewWriterRuntime(deps, path, stdout, opts...), a different constructor with
  a different options path (WithDocumentMode, WithPrincipal, WithCache, WithTimeout).


  So it proves "elevatedDeps populates the struct", which is one layer short of the claim. The gap is
  not hypothetical: if NewWriterRuntime ever stripped or transformed deps for document mode -- precisely
  the fix for RR-DOCWRT -- this test would keep passing while the feature silently stopped working.


  Either narrow the comment to what it proves, or route the test through the real engine.'
resolution: |
  Narrowed the comment to what the test proves, and added the test that covers the real path.
  
  TestElevatedDeps_GrantsBypassBinding now says explicitly that it builds lua.NewWriter DIRECTLY, that this is NOT the production path, and that it therefore pins the deps-to-bindings mapping rather than end-to-end wiring — including the specific hazard (if NewWriterRuntime transformed deps for document mode, this would keep passing).
  
  Both tests are kept, with the comment explaining why: this one localizes a failure to elevatedDeps, TestElevatedRender_ReadsHiddenEntityAndAudits proves the feature actually works.
severity: significant
status: addressed
---
