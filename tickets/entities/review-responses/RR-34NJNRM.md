---
id: RR-34NJNRM
type: review-response
title: Docs claimed automations follow the scope and caller access
finding: Same-entity on-update automation sets run inside the elevated write past the scope; the guide and godoc said automations run with the caller's normal access.
severity: significant
resolution: GUIDE-data-entry and the ScopedUpdates godoc now say same-entity sets are part of the elevated write and only cascades (create_entity and run_script) run with the caller's access. GUIDE-acl-security says the same.
status: addressed
---
