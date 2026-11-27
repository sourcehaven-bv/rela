---
id: RR-OFNZHVL
type: review-response
title: End-to-end host-path leak test replaced by a unit test
finding: The reload SSE frame was no longer checked for the project root.
severity: minor
resolution: 'Restored TestReloadConfigErrorOmitsHostPath with an allow_acl_bypass: true fixture, which is still refused on load.'
status: addressed
---
