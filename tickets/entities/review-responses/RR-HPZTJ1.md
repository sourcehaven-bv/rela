---
id: RR-HPZTJ1
type: review-response
title: Trailing/leading whitespace in a non-blank membership_relation silently matches nothing, no diagnostic
finding: 'membershipRelation() returns the raw field verbatim when non-blank. A YAML value like ''heeft_rol '' (trailing space) is non-blank, so it reaches store.RelationQuery{Type:''heeft_rol ''} and matches zero edges. Fail-closed (no security hole) but a silent ''my groups don''t work'' with no diagnostics. Given the whole PR is about advisory hardening warnings, omitting this is inconsistent. Fix: TrimSpace the effective value in the accessor (also makes isBlank handling cleaner), or have Validate warn on leading/trailing whitespace.'
severity: minor
resolution: 'membershipRelation() now strings.TrimSpace-es the field: a non-blank trimmed value is returned, else the default. So ''heeft_rol '' resolves to ''heeft_rol'' (matches the intended relation) and pure-whitespace still collapses to member-of. Added AC6 case ''configured with padding'' (''  heeft_rol  '' → ''heeft_rol'').'
status: addressed
---
