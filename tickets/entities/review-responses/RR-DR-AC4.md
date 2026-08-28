---
id: RR-DR-AC4
type: review-response
title: AC4's test passes vacuously and cannot distinguish 'fallback removed' from 'fallback missed this
  spelling'
finding: 'AC4 wants ''root-level custom.css no longer served or injected'', tested by writing custom.css
  at the project root and asserting /_custom/custom.css 404s. VERIFIED this proves nothing: after the
  change openCustomEntry resolves under custom/, so a root-level file is simply not in that tree and 404s
  regardless of whether a root fallback still existed elsewhere. The assertion is identical in force to
  AC3''s traversal cases, and would keep passing if a resurrected fallback were live but reached by a
  different spelling.'
severity: significant
status: addressed
resolution: 'Folded into TKT-IWMETE and PLAN-6VVJJZ before implementation. Make the negative observable
  by DISCRIMINATION rather than absence: write BOTH <root>/custom.css containing ''ROOT-VERSION'' and
  <root>/custom/custom.css containing ''FOLDER-VERSION'', then assert the served body is FOLDER-VERSION
  and never contains ROOT-VERSION - a test a resurrected fallback would actually fail. Add the injection
  half: with ONLY the root-level file present, assert selectShell returns variants.plain (shell byte-identical
  to the no-customisation case). As written AC4 and AC5 assert nearly the same thing.'
---

Raised by `/design-review` of TKT-IWMETE before implementation.
