---
id: RR-S7U5
type: review-response
title: 'Architect #3: buildManager two-phase init'
finding: Workspace built Manager via a second-phase buildManager() method after the New()/NewForTest() flow assigned the real store. Risked binding Manager to the placeholder memstore if reordered.
severity: significant
resolution: Refactored newWorkspace to take store as a parameter; folded Manager construction directly into newWorkspace; deleted buildManager(). Single-phase construction, no placeholder memstore involved.
status: addressed
---
