---
id: RR-FF7Q
type: review-response
title: _actions on empty list response may show Create button for denied principal
finding: 'On a DenyAll list response, data is `[]` (good) but `resp.Actions = a.computeCollectionActions(r.Context(), typeName)` still runs (api_v1.go:456). computeCollectionActions consults the same Request so this MIGHT compute create:false correctly today, but the plan doesn''t pin it. Add AC assertion: ''DenyAll list response has _actions.create == false''. Otherwise a denied principal sees an empty list with a Create button (UX bug at minimum, signals the type exists in the metamodel at worst).'
severity: minor
reason: Moved to TKT-VMD8 (covers list response shape). AC4 pins _actions.create == false on DenyAll list.
status: deferred
---
