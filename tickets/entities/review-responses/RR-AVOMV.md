---
id: RR-AVOMV
type: review-response
title: List-typed display_property silently renders "[foo bar]"
finding: 'validateDisplayProperty in loader.go only checks (a) whitespace and (b) existence — not list/map types. Setting display_property to a list-typed property like ''tags'' makes DisplayTitle call fmt.Sprintf("%v", val) on []interface{}, producing strings like "[foo bar]". Empty lists yield "[]" — non-empty, so the ID-fallback never triggers. Same shape as RR-9CW5N but for collections. Map-typed values render as "map[a:1]". None of this is tested. Fix at load time: reject list-typed (and ideally map-typed) properties in validateDisplayProperty, citing the entity, the offending prop, and the contract ("display_property must point at a scalar property").'
severity: significant
resolution: 'Added type+list rejection to validateDisplayProperty. List-typed properties (List: true) fail load with ''cannot render as a display name''. New test TestParse_DisplayPropertyList pins the behavior.'
status: addressed
---
