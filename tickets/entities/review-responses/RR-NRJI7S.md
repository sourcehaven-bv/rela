---
id: RR-NRJI7S
type: review-response
title: About body used pre-wrap on rendered-markdown HTML
finding: '.about-body had white-space: pre-wrap, but the body is renderMarkdown HTML (block elements) — pre-wrap preserves source newlines between block tags, doubling vertical gaps. Right for raw text, wrong for rendered markdown.'
severity: minor
resolution: 'Dropped pre-wrap; added :deep(p) margins for clean paragraph spacing. Verified: markdown renders correctly (emphasis shows, no doubled gaps).'
status: addressed
---
