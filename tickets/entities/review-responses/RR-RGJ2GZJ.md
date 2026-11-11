---
id: RR-RGJ2GZJ
type: review-response
title: Tenants that are not resident never fire timers
finding: The worker runs only while a tenant is resident (DefaultMaxResident is 4). A tenant nobody opens never has its escalations or retries run in multi-tenant rela-server.
severity: significant
reason: Opening tenants from a control-plane sweep is a design of its own (resident budget, failing runs that would reopen a tenant in a loop, and the same gap for realtime timers). Filed as TKT-ZOU5BCT. GUIDE-flows states that a tenant's worker runs while the tenant is loaded, and the report asks the user to confirm this placement.
status: deferred
---
