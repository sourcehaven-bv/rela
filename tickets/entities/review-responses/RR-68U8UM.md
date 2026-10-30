---
id: RR-68U8UM
type: review-response
title: '"never shows" warning is not strictly true'
finding: 'AC 3''s warning "the type''s button never shows" when every entry is buttoned: presetViews sends every row to the type button when _actions.entries is absent (frontend/src/components/entity/channels/presetViews.ts:57), which happens against an older server. Word it "does not show" or "is not shown by this release''s web app".'
severity: nit
resolution: The warning is dropped, because with inheritance decision_button applies to presets even when every entry has a button. The older-server case is listed as an edge case.
status: addressed
---
