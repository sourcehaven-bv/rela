---
id: RR-JQ008B0
type: review-response
title: Duplicated provisioning validation and two Provisioners
finding: Provisioning config validation was duplicated between cmd/rela-server and tenanthost, and the wiring built two separate tenant Provisioners.
severity: minor
resolution: 'Fixed in 13cd61e18: validation lives in one place and the wiring shares one tenantWriter. Pinned by TestProvisionConfig.'
status: addressed
---
