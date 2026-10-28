---
id: RR-RAW9PC
type: review-response
title: Upgrading note omits that editors lose conflict resolution
finding: Roles with update could resolve conflicts before; after the change they need git:sync. The guide's Upgrading note mentioned only the sync button.
severity: significant
resolution: The Upgrading note now says to grant git:sync to roles that used the sync button or resolved merge conflicts. docs/ regenerated.
status: addressed
---
