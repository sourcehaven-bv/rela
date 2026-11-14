---
id: RR-GT629C0
type: review-response
title: flowConfigPolicy fails boot on a schema-mismatched acl.yaml
finding: 'Stricter than newRealtimeConfig: under an injected ACL a stale acl.yaml would stop rela mcp even when flows name no roles.'
severity: significant
resolution: Both now use loadConfigPolicy, which validates for type expansion and tolerates a mismatch; pinned by TestFlowEngine_InjectedACLConfigPolicyEdges.
status: addressed
---
