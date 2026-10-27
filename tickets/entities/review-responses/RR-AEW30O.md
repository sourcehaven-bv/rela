---
id: RR-AEW30O
type: review-response
title: Stored user palette was not validated on load
finding: palette_service.load parsed .rela/palette.yaml without ValidatePalette; operator colour names become custom property names in _rela.css.
severity: significant
resolution: load() now validates and returns an error; the RR-OA4A surface path reports it.
status: addressed
---
