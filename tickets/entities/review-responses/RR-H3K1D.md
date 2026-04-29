---
id: RR-H3K1D
type: review-response
title: /Jan/ test assertion fails on non-English CI runners
finding: 'The toMatch(/Jan/) assertion passes today on en_US.UTF-8 GitHub runners but fails on fr-FR (janv.), ja-JP (1月), ru-RU (янв.). Ubuntu base image locale defaults have changed before. Fix: pass an explicit locale parameter to formatDate (e.g. ''en-US'') and assert with toBe(''Jan 15, 2024''); production callers pass undefined so behavior is unchanged in the app, but tests are deterministic.'
severity: critical
resolution: formatDate now accepts an optional locale parameter. Tests pass 'en-US' and 'en-GB' explicitly and assert toBe('Jan 15, 2024') / toBe('15 Jan 2024'). Production callers still pass undefined so app behavior follows host locale. The two locale-dependent toMatch(/Jan/) assertions in formatValue/formatCellValue tests were replaced with toMatch(/15/) and toMatch(/2024/) — the day component changes under TZ shifts and is independent of locale name strings.
status: addressed
---
