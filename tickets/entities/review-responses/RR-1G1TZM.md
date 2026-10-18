---
id: RR-1G1TZM
type: review-response
title: Promote reverts to none on errors raised after CreateEntity already committed
finding: C3 step 11 treats every Promoter error as "nothing committed", but entitymanager.CreateEntity returns (nil, err) after createCore persisted the entity when writeAutomationProperties or Cascade.Process fails (manager.go ~950-975). Reverting pending to none then lets the next accept create a second entity, which breaks AC14. Revert only on errors known to come before the save (acl.ForbiddenError, affordances.FieldWriteError, validation, face and custom-ID errors), leave everything else pending for the D1 release path, and run Promote on context.WithoutCancel with a timeout so a client disconnect cannot abort it halfway.
severity: critical
resolution: 'Per Jeroen D1: on pg and sqlite the entity, the relation and the action mark commit in ONE store transaction, so automation or cascade errors after commit return warnings and never revert. On fs the action reverts only on error classes raised before the save (ACL, field write, validation, face, custom id); any other error leaves it pending for release. Promote runs on context.WithoutCancel with a timeout.'
status: addressed
---
