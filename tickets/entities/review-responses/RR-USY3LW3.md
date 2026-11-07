---
id: RR-USY3LW3
type: review-response
title: Security guide lacks elevated data-entry action section
finding: GUIDE-acl-security had no section on what an elevated action permission grants and had stale admin handle and allow_acl_bypass text.
severity: minor
resolution: Added Elevated data-entry actions (what the permission grants; the caller picks the row; the script must check ownership and state; ceilings apply). Rewrote the stale paragraph per surface. The data-entry example now checks owner and draft status.
status: addressed
---
