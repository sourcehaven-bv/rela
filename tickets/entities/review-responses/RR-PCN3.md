---
id: RR-PCN3
type: review-response
title: Docs missing operator reference link and asymmetry note
finding: 'docs/data-entry.md URL Sync section mentions ''See the API reference for the full list'' of operators but doesn''t link. Also doesn''t document the in/ne vs other-ops asymmetry: in/ne join all repeated values, others use last-write-wins. Add the link and a one-liner about the asymmetry.'
severity: nit
resolution: 'docs/data-entry.md URL Sync section: (a) inlines the full operator list (eq/ne/contains/in/lt/lte/gt/gte) rather than leaving it as an unlinked reference, (b) documents the fail-closed behavior for unknown operators, (c) documents the in/ne vs other-op asymmetry for multi-value (repeated keys are joined only for in/ne, others are last-write-wins).'
status: addressed
---
