---
id: RR-ZLV3VZJ
type: review-response
title: "Provisioning on open could write for every visitor"
finding: "POST _me on each load would write for every user."
severity: significant
resolution: "Addressed: the SPA posts only when GET says provisionable, and a second POST creates nothing (TestProvision_MePost)."
status: addressed
---
