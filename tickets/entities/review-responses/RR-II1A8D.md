---
id: RR-II1A8D
type: review-response
title: Exported Render used only by tests
finding: Render had no caller outside tests after the refactor.
severity: nit
resolution: Removed; tests use a local helper over Build and Mermaid.
status: addressed
---
