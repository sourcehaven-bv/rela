---
id: RR-7GN3LV
type: review-response
title: "Validator ReadDeps.Tracer stayed raw: trace bindings could walk hidden entities (by-construction gap)"
severity: significant
status: addressed
finding: >-
  Code review of the gated-analyze change found that while the validator's
  ReadDeps.VisibleReader was gated, its ReadDeps.Tracer stayed RAW (`trc`). A Lua
  validation rule can call rela.trace_from / trace_to / find_path, which read
  ReadDeps.Tracer and walk into hidden entities. NOT exploitable today — only
  because a Lua rule's returned message is dropped by validator.CheckRuleFull
  (violations carry EntityID + Detail, not the Lua message) and violations
  attach to the gated candidate set, so nothing hidden reaches the wire. But the
  fix's thesis is "close by construction, not by lucky output shape": this seam
  was safe only by a downstream truncation, and the deleted visibleAnalysisIssues
  output filter was the removed backstop. A future change that enriches a
  violation Detail from trace output would silently reopen the exact leak class.
resolution: >-
  Added `lateGatedTracer` (the tracer.Tracer counterpart of lateGatedReader):
  resolves App.scriptTracer per call, which returns the VisibleTracer decorator
  (prunes hidden nodes, redacts, fails closed under a Declarative policy;
  raw under NopACL). Wired into BOTH the validator's ReadDeps.Tracer AND
  analyzeService.tracer (belt-and-braces; the orphans re-load was already gated
  via svc.reads.GetEntity, now the tracer is gated too). Added a safety comment
  on analyzeOrphans pinning the "re-load every tracer id through the gated reader
  before emitting" invariant so the next editor can't reopen it. The underlying
  VisibleTracer gating is already covered by internal/lua/aclreads_test.go
  (trace_from under ACL); no on-wire end-to-end leak test is possible here
  because the Lua message never reaches the wire — the gating is defense-in-depth
  making the seam safe by construction rather than by output shape. Full suites
  green under -race; lint/plimsoll/arch clean.
---
