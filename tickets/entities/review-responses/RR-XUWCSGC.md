---
id: RR-XUWCSGC
type: review-response
title: Provision flags silently ignored outside tenant mode
finding: -tenant-provision and related flags were accepted and ignored when -tenant-mode was off, suggesting provisioning was configured when it was not.
severity: minor
resolution: 'Fixed in 13cd61e18: the flags are refused at startup outside tenant mode. Pinned by TestRefuseTenantOnlyFlags.'
status: addressed
---
