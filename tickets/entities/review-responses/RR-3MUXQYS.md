---
id: RR-3MUXQYS
type: review-response
title: One broken tenant schema failed the whole due-work sweep
finding: One tenant with a broken or old schema made the due query fail, which failed the whole sweep pass for every tenant.
severity: significant
resolution: The sweep reads which due-work columns each schema has through pg_attribute and leaves missing parts out, gives each tenant a 5s timeout, and logs and skips a tenant whose due work cannot be read (TestClusterResolver_DueTenants). Commit f2f2dac83.
status: addressed
---
