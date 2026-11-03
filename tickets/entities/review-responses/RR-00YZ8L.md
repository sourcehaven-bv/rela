---
id: RR-00YZ8L
type: review-response
title: Supported-language list can drift between SPA, Go and locale files
finding: The plan has a SUPPORTED constant in the SPA and SupportedUILanguages in Go, both hand-written next to the locale files.
severity: significant
resolution: 'Plan: the SPA derives its list from the imported locale objects; a Go test compares SupportedUILanguages with frontend/src/i18n/locales/*.json on disk.'
status: addressed
---
