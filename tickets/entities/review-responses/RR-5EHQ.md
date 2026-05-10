---
id: RR-5EHQ
type: review-response
title: Extract maxThemeUploadBytes constant
finding: handlers_theme_package.go:53-54 — `ThemePackageMaxBytes+16*1024` repeated for MaxBytesReader and ParseMultipartForm. Mirror handlers_theme.go::maxLogoUploadBytes by introducing maxThemeUploadBytes near the constant definition so they can't drift.
severity: nit
resolution: Extracted maxThemeUploadBytes constant near ThemePackageMaxBytes. Both MaxBytesReader and ParseMultipartForm now use the constant; mirrors the maxLogoUploadBytes pattern.
status: addressed
---
