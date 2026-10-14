---
id: RR-LYV3VZ
type: review-response
title: An everyone role holding the verb makes other roles' blocks inert
finding: 'Every principal holds everyone. Under the union rule, everyone with read on a type and no visible: block shows every field to everyone, so a restricting role''s visible: block no longer hides anything; the same holds for update with fields:/options:. Existing deployments that layered blocks on top of everyone widen silently on upgrade. Add an aclaudit finding and a load-time warning, an upgrade note in the docs, and list golden changes.'
severity: significant
resolution: Added aclaudit rule A14-everyone-opens-field-block (high) and an Upgrading note in GUIDE-acl-security and GUIDE-acl-overview. Pinned by TestAudit_A14_EveryoneOpensFieldBlock.
status: addressed
---
