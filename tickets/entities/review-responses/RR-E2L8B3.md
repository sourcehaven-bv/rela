---
id: RR-E2L8B3
type: review-response
title: 'Missing source reported as source: entry'
finding: 'validateSectionHideEmpty reported a section with no source as ''source: entry is never empty''. At runtime such a section reads a collection that never exists and would always hide.'
severity: significant
resolution: 'Split into two messages: one for source: entry and one for a missing source. The guide lists both.'
status: addressed
---
