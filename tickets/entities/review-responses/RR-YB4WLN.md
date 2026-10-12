---
id: RR-YB4WLN
type: review-response
title: Corpus would test the test wiring
finding: newAppFromParts copies NewApp by hand and hardcodes ungatedAuthorizer; drift in NewApp (PR 6) would not show.
severity: critical
resolution: 'Plan: build the corpus app through NewApp with appbuildtest services and SelectCommandAuthorizer.'
status: addressed
---
