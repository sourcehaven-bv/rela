---
id: RR-84YM
type: review-response
title: checkZipExpansion uint64-wrap defeats defense-in-depth
finding: 'theme_package.go:158-173 — `total += f.UncompressedSize64` wraps on crafted zip64 entries (single value >= 2^64 - 5MB collapses below the cap). The post-loop `int64(total)` cast also truncates. The doc comment claims overflow is caught, but it isn''t. Only `LimitReader` in readZipEntry actually saves us. Use a saturating add and stay in uint64 throughout: `if f.UncompressedSize64 > ThemePackageMaxBytes || total > ThemePackageMaxBytes-f.UncompressedSize64 { return errZipUncompressed }`.'
severity: significant
resolution: 'checkZipExpansion now uses saturating add: any entry whose UncompressedSize64 > ThemePackageMaxBytes or would push the running total over the cap is rejected before total wraps. Stayed in uint64 throughout (no int64 cast); guard the multiply by clamping compressedTotal first.'
status: addressed
---
