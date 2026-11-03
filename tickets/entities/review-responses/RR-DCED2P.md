---
id: RR-DCED2P
type: review-response
title: A14 runs only on demand
finding: A14 is reported only by rela acl audit. Log it at acl.yaml load like warnQueryScopes, and consider roles everyone effectively holds.
severity: minor
resolution: 'appbuild logs every opened block as a warning when it loads acl.yaml (warnEveryoneOpensFieldBlocks next to warnQueryScopes), sharing acl.EveryoneOpenedFieldBlocks with the audit. Only everyone is considered: whether every principal holds another role depends on assignments, edges and IdP claims, not on the policy, so it cannot be judged at load.'
status: addressed
---
