---
id: RR-M02AYI
type: review-response
title: Module-level t() freezes the language
finding: Option arrays and label maps defined at module scope (status labels, menu entries) would call t() once at import and never follow a language switch.
severity: significant
resolution: Module-scope tables hold keys, not text; t() runs in a computed or at render. Code review checks for t() at module scope.
status: addressed
---
