---
id: RR-ZK28ZP
type: review-response
title: FormRelation.Span precedent is json:"-" — plan draws the wrong lesson for a key that must serialize
finding: 'The plan cites FormRelation.Span as the precedent for the new config key, but Span is tagged json:"-" (config.go:560) — captured for validation and deliberately NOT sent to the SPA. keep_on_add_another must serialize. A reader following the cited precedent literally ships a key the frontend never sees. Also: checkUnknownKeys only walks top-level keys and unmarshal is non-strict, so a typo''d key is undiagnosable — the struct tag is not a complete mitigation.'
severity: significant
status: addressed
resolution: >-
  Plan step 0 corrected: take Span's yaml lesson, not its json tag (Span is json:"-"). keep_on_add_another must serialize. Added the note that checkUnknownKeys is top-level-only so a typo'd key is undiagnosable, with the docs entry as the discoverability backstop.
---
