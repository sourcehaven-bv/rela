---
id: RR-6XQYYD3
type: review-response
title: Template variables incomplete; unknown placeholders pass silently
finding: Interpolate also supports entity.id and entity.type; unknown placeholders such as subject.title are written literally; inline lua uses InterpolateSafeOnly.
severity: minor
resolution: 'Plan: list entity.id/entity.type; refuse unknown placeholders at load and in the migration; scripts keep safe-only interpolation.'
status: addressed
---
