---
id: RR-0T4NIZ
type: review-response
title: acl audit passes face block keys the resolver refuses
finding: 'B11 checked only create/update/delete lists, so a fields: key with an undeclared face or a visible:/relations: face key passed rela acl audit but failed affordances.New at startup.'
severity: significant
resolution: 'Added checkFaceBlockKeys (B11) for fields:/options: keys with undeclared faces and any face key in visible:/relations:. TestAudit_B4B5_FaceKeys covers all three.'
status: addressed
---
