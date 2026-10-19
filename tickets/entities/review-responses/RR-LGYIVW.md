---
id: RR-LGYIVW
type: review-response
title: Decision 3 lets --run-as impersonate humans and system:realtime trigger posts
finding: 'ValidateRunAs with anySystemIdentity accepts plain users and every system: name. sameAuthor (authz.go) ignores tool and asserted, so `--run-as alice` passes alice''s edit_own, delete_own and creator read roles, and `--run-as system:realtime` forges T5 trigger posts that agents may trust more. Jeroen''s decision 2 says "job identity such as system:cron". Restrict to system:/integration: names and refuse rela''s own (system:realtime, scheduler, automation, provisioner).'
severity: significant
resolution: 'Per Jeroen: only system: and integration: names are accepted. rela''s own system:realtime, system:scheduler, system:automation and system:provisioner, plain users and unknown are refused.'
status: addressed
---
