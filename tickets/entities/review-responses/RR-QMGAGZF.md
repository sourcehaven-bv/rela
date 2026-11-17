---
id: RR-QMGAGZF
type: review-response
title: ErrTenantInUse made every poll retry and spend rate-limit tokens
finding: When another server was creating the same tenant (ErrTenantInUse), every status poll started a new attempt and spent rate-limit tokens.
severity: significant
resolution: The provisioning job is held as running for 10s after ErrTenantInUse, so polls during that time do not retry (TestAutoProvisioner_InUseHolds). Commit f2f2dac83.
status: addressed
---
