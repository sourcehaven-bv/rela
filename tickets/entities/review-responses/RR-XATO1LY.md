---
id: RR-XATO1LY
type: review-response
title: No test for deactivating an already provisioned member
finding: 'Each clear case started from an empty workspace, so the real #2117 sequence (active then inactive) was not tested.'
severity: minor
resolution: 'Added TestIdpSyncAction_DeactivatesProvisionedMember: active run asserts [founder], inactive run asserts an empty list, one person remains with unchanged email.'
status: addressed
---
