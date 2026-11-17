---
id: RR-1E63BRL
type: review-response
title: 'Security: trigger placeholders and conditions read ACL-hidden values'
finding: 'A config:edit holder can add a trigger whose {{new.x}} placeholders or when/where conditions read a property hidden by visible: or a row when: and copy it into a record they can read. aclWriteReferences judges only writes.'
severity: critical
resolution: 'appbuild.CheckTriggerACL judges each added or changed trigger against the parsed acl.yaml (checked against the draft schema) and the editor''s request; configedit calls it through the TriggerChecker interface before any early return. Reads in on.property, where, check where, when attributes and {{new.x}}/{{old.x}} placeholders are refused when FieldRestricted or a scope grant hides the field; when may not follow relations or pass the record whole; a created record must be readable only by readers of the source, also through cascades; relate.to may not be set and pile names may not hold placeholders; Interpolate no longer expands substituted text. Tests: TestCheckTriggerACL, TestConfigure_FlowsTriggerACL, TestInterpolate_SubstitutedTextIsNotExpanded, TestPropertyReads_AgreesWithInterpolate.'
reason: The old schema.yaml automations editor opened the same placeholder and condition keys with no ACL check at all; this change keeps that surface and adds write checks. Closing the read route needs a check against the parsed acl.Policy (visible-gated fields are the ones NOT named and row gates depend on when:), which configedit cannot import and the name match cannot express. Tracked in TKT-CB1YV7D. Until then the data-entry and acl-security docs state that config:edit is read-all access and must only be granted to people who may read all data.
status: addressed
---
