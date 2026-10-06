---
id: RR-G0POGM
type: review-response
title: Quoted-string rule counted comments and class modifiers as definitions
finding: cssCustomProperties.test.ts read any quoted --name as a definition, comments included, so a name only mentioned in a comment (for example --font-size-md in scales.css) would pass the guard.
severity: significant
resolution: Comments are stripped before scanning. A definition now needs a colon after the name (CSS declaration or object key) or a setProperty('--x') call. The narrower rule surfaced one more undefined use, --muted-text in RelationPicker, now fixed.
status: addressed
---
