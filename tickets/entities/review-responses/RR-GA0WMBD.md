---
id: RR-GA0WMBD
type: review-response
title: Lazy polls use up the provisioning start rate while another node provisions
finding: When another server held the tenant lock, every lazy poll started a run that ended with ErrTenantInUse and took a start-rate token, so polling clients could exhaust the process-wide provisioning budget.
severity: minor
resolution: 'Fixed in 3a5b2d9ff: a run that ends with ErrTenantInUse is remembered for 5 seconds. During that hold a request answers 202 provisioning and the webhook 202 without starting a run or taking a token. Pinned by provision_test.go.'
status: addressed
---
