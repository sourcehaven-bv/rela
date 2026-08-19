---
id: RR-DOCAUDIT
type: review-response
title: No test proves an elevated render lands an audit row; the tests that look like they would are neutered
finding: 'document.go:190-193 calls the acl-bypass-read trail "the exact gap TKT-ACSBSA closed; it must
  not reopen on a new surface". Nothing in internal/dataentry tests it -- the only audit tests live in
  internal/lua, internal/script and internal/audit, none of which exercise the document path.


  Worse, the HTTP tests that appear to cover it cannot. withFakeDocRenderer (standalone_document_handler_test.go:26-32)
  calls newTestService(...) which passes nil for the elevation func (document_script_test.go:166) and
  overwrites app.documents. So TestElevatedDocument_PermittedPrincipalRenders renders through a fake with
  s.elevation == nil, elevatedDeps short-circuits, and no elevation happens at all. Its assertions (200,
  callCount == 1) pass identically whether elevation works, silently no-ops, or was never wired.


  That is the same failure mode test_helpers_test.go:168-170 explicitly warns about in a comment I wrote
  this session -- reintroduced downstream of rebindApp.


  Fix: one integration test with a real script.Engine and a capturing audit.Audit asserting (a) the script
  reads an entity the gated reader denies and (b) exactly one OpACLBypassRead row lands. The reviewer
  confirmed both hold, so this pins working behavior rather than chasing a bug.'
resolution: |
  Added TestElevatedRender_ReadsHiddenEntityAndAudits and TestUnelevatedRender_CannotReachHiddenEntity: real Lua scripts through a real script.Engine, exercising ExecuteStandaloneDocument -> runDocumentScript -> NewWriterRuntime.
  
  The design makes the assertion meaningful rather than incidental: the ordinary VisibleReader is visibility.DenyReader{}, so the hidden entity is reachable ONLY through the elevated handle. A capturingAudit sink then asserts exactly one acl-bypass-read row. The negative test asserts the unelevated render FAILS (no bypass_acl binding) and produces zero audit rows.
  
  Mutation-tested to confirm it is not another test that passes for the wrong reason: forcing elevatedDeps to ignore cfg.Elevated makes it fail with "elevated render failed (the elevated reader did not arrive): attempt to call a non-function object"; restoring makes it pass.
  
  The neutered HTTP tests were left as-is deliberately — they cover the GATE (403 vs 200, renderer not invoked on deny), which is what they were written for and which they do correctly. The integration tests cover the wiring they cannot.
severity: significant
status: addressed
---
