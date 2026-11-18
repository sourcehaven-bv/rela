---
id: RR-6KYGN2F
type: review-response
title: 'Design: trigger outputs write without ACL checks'
finding: 'Cascade writes are ungated (entitymanager cascadeWrite). Opening relate/create/set outputs lets a config:edit holder write records acl.yaml depends on (member-of, principal_property, when: properties) and so grant itself roles. The plan''s residual-risk line claimed the ACL gates every write.'
severity: critical
resolution: 'appbuild.CheckTriggerACL judges each added or changed trigger against the parsed acl.yaml and the editor''s own grants. It refuses a trigger that writes what acl.yaml decides access by (the principal property, a property or relation a grant''s when: reads, an options: field, the membership relation, a principal record) or what the editor could not write directly. The problem code is acl_trigger. An unchanged trigger is judged only for what a schema change adds. Tests: TestCheckTriggerACL, TestCheckTriggerACL_FailsClosed and TestSave_FlowsRefusals (refused by the ACL check).'
status: addressed
---
