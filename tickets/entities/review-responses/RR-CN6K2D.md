---
id: RR-CN6K2D
type: review-response
title: Mermaid sanitizer flattens foreignObject label markup
finding: 'Single-pass DOMPurify with the SVG profile unwraps the XHTML inside foreignObject so only text survives: br multi-line labels collapse to one word; the .nodeLabel styling hook is lost; bare text is not valid foreignObject content. Allowing the HTML tags is worse — DOMPurify 3.4 force-removes an allowed HTML element under an SVG parent.'
severity: critical
resolution: 'Two-pass: an uponSanitizeElement hook on a dedicated DOMPurify instance lifts each label during the SVG pass; each label is then sanitized under the HTML profile and written back. Structural unit tests assert .nodeLabel / p / br / style survive; e2e/tests/mermaid.spec.ts renders real mermaid in a real browser and checks the same.'
status: addressed
---
