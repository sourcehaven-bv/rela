---
id: RR-WYEQBD
type: review-response
title: Docs overclaim coverage
finding: Section said every write surface.
severity: minor
resolution: acl-security.md rewritten with an explicit not-covered list (UpdateEntity callers, actions, CLI, relations, attachments) and FieldWriteGate godoc names UpdateEntity/RecreateEntity as ungated.
status: addressed
---
