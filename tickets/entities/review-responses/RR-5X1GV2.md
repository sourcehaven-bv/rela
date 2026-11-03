---
id: RR-5X1GV2
type: review-response
title: Visibility rule duplicated outside the compiled grants
finding: The plan puts the rule on acl.Policy over raw RoleDef.Visible, while FieldVerdicts reads the compiled grants in affordances. Two encodings of "is this field hidden" can drift. Implement it on PolicyResolver over r.grants, take the conferrable roles from an acl accessor, and add a differential test against FieldVerdicts.
severity: significant
resolution: 'Removed the acl.Policy rule. PolicyResolver.FieldVisibleOnEveryRow (internal/affordances/fieldfilter.go) reads the compiled grants (declaredVisible, program == nil) that FieldVerdicts reads, plus acl.Policy.ConferrableRoles. TestFieldVisibleOnEveryRow_NeverContradictsFieldVerdicts checks that FieldVerdicts never hides an allowed property on any row, across local roles and when: outcomes.'
status: addressed
---
