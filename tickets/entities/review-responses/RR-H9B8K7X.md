---
id: RR-H9B8K7X
type: review-response
title: A valid tag that is not a UI language
finding: languageTag accepts any BCP 47 shape, for example fr. The SPA might then show a language it has no messages for.
severity: minor
resolution: 'Addressed: the meta only seeds projectLanguage, which goes through resolveLanguage like the config value, so an unsupported tag falls back the same way as after the config loads.'
status: addressed
---
