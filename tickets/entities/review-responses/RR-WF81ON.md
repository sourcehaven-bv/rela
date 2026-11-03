---
id: RR-WF81ON
type: review-response
title: Login grammar admits invisible format characters
finding: 'The `@x` rule refuses whitespace, control characters, backtick and NUL. Unicode format characters (Cf: U+200B, U+2060, bidi marks) are neither whitespace nor control in Go, so `` `@jeroen​` `` renders a chip that reads as @jeroen while it targets a key nobody holds. Fix: also refuse unicode.Is(unicode.Cf, r) and non-NFC input, matching the spirit of entity.ValidateID''s homoglyph argument. Document that login matching is exact and case-sensitive, as acl.yaml assignments are.'
severity: minor
resolution: Login grammar refuses Unicode Cf and non-NFC input; matching documented as exact and case-sensitive; TestClassifySpans rows.
status: addressed
---
