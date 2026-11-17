---
id: RR-6KYGN2F
type: review-response
title: 'Design: trigger outputs write without ACL checks'
finding: 'Cascade writes are ungated (entitymanager cascadeWrite). Opening relate/create/set outputs lets a config:edit holder write records acl.yaml depends on (member-of, principal_property, when: properties) and so grant itself roles. The plan''s residual-risk line claimed the ACL gates every write.'
severity: critical
resolution: 'aclWriteReferences refuses an added or changed trigger that writes a name acl.yaml mentions or the membership relation or user entity type (code acl_reference). Tests: TestSave_FlowsRefusals and TestSave_UnchangedTriggerKeepsACLReference.'
status: addressed
---
