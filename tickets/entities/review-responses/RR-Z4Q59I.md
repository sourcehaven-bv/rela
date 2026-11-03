---
id: RR-Z4Q59I
type: review-response
title: 'Table list cards: unchecked columns and sort; silent limit clamp; hidden cells look empty'
finding: Columns and sort were not checked against the list type, a limit above 100 was clamped silently, and _redacted was dropped in the SPA.
severity: minor
resolution: validateTableCard checks columns, sort and limit (max 100); the SPA carries _redacted and renders a Hidden marker. Tests added for each.
status: addressed
---
