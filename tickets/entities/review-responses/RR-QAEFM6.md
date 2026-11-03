---
id: RR-QAEFM6
type: review-response
title: '"Give the lead and dpo roles field write in testdata/acl.yaml" mixes up realtime roles and ACL roles'
finding: '`lead` (`from: entity.owner`) and `dpo` (`permission: privacy.officer`) are realtime roles. acl.yaml grants to ACL roles (`officer`, `breach-bot`, `auditor`; `realtimecfg/testdata/acl.yaml`). Change: name the ACL grants. For example, `officer` gets update and field write on `datalek.owner` for dpo. Lead needs an ACL role or an owner-conditioned rule that its user holds. Check that the new grants do not change the existing grant-check and run_as test outcomes over the shared testdata.'
severity: minor
resolution: 'testdata/acl.yaml grants ACL roles: new handler (assigned USR-1) and officer get update on datalek; the run_as grant-check tests re-run unchanged.'
status: addressed
---
