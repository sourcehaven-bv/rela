---
id: RR-DOCGREP
type: review-response
title: toDocumentRenderConfig is the single switch enabling elevation, with nothing pinning its callers
finding: 'Elevated: true has exactly one producer -- toDocumentRenderConfig (handlers_document.go:23).
  Its two callers are both correctly gated (standalone_document_handler.go:196, api_v1.go:2160), and the
  reviewer verified exhaustively that no CLI, MCP, scheduler, calfeed or autocascade path reaches cfg.Documents,
  and that both export paths leave Elevated at its zero value.


  So it is correct today. But the coupling is a coincidence, not an invariant: a third handler calling
  toDocumentRenderConfig without the gate would compile and silently elevate.


  This package already has the pattern for exactly this. TestNavFilterStaysPresentational greps permitsNavEntry(
  / permitsGatedUIElement( against a short allow-list, and internal/dataentry/CLAUDE.md documents it:
  "keep the grep guard''s allow-lists short... adding a caller means widening a list, which is a deliberate
  argued exception."


  A parallel grep test pinning toDocumentRenderConfig( to its two call sites costs ~15 lines and converts
  the coincidence into an enforced invariant. Given this function is the single switch that turns on raw
  ACL bypass, it warrants the guard at least as much as the nav filter does.'
resolution: |
  Added `toDocumentRenderConfig(` as a third needle to the existing TestNavFilterStaysPresentational guard in lint_test.go, rather than writing a parallel walk — the table was already the right shape.
  
  Allow-list: standalone_document_handler.go, api_v1.go (the two gated callers) and handlers_document.go (the definition). The `why` string states the invariant: it is the only switch that enables elevated reads, and every caller must pass gateElevatedDocument first.
  
  Verified the guard bites: adding a probe caller in document.go fails the test with an actionable message naming the file and the rule; removing it passes again.
severity: significant
status: addressed
---
