---
id: RR-WRFCW5
type: review-response
title: Whitespace-only membership_relation must default, not pass through (same match-all foot-gun as blank)
finding: 'The plan handles empty string but not whitespace-only (e.g. membership_relation: ''   ''). A whitespace relation name is not a real relation type and would behave unpredictably (and a blank-after-trim could hit the Type=="" match-all path depending on store handling). Validate already has isBlank() and uses it to reject blank InheritRolesThrough/RoleRelations keys. The effective-name helper should treat isBlank(MembershipRelation) as ''unset'' and return defaultMembershipRelation, so '''' and ''   '' and a tab all collapse to member-of. Add a test for whitespace-only → default.'
severity: minor
resolution: 'Plan revised: membershipRelation() uses isBlank() so "", whitespace, and tab all collapse to member-of. AC6 added: table test over {"", "   ", "\t"} → "member-of".'
status: addressed
---
