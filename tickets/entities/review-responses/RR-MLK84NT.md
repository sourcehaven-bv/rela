---
id: RR-MLK84NT
type: review-response
title: "Suspended was inferred from what it is not"
finding: "The host called any inactive, non-provisioning tenant suspended, so a new inactive state would be reported as suspended."
severity: minor
resolution: "Addressed: the resolver adds ErrTenantSuspended and the host answers tenant_suspended only for it. Pinned by TestClusterResolver_Archived and TestHost_FailsClosed."
status: addressed
---
