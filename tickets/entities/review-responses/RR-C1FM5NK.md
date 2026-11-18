---
id: RR-C1FM5NK
type: review-response
title: Tenant with due work that never cleared was reopened every cooldown
finding: A tenant whose due work never cleared (for example a run that keeps failing) was opened again after every cooldown, forever.
severity: significant
resolution: The cooldown doubles while the tenant's DueAt is unchanged, up to 6h (sweepMaxCooldown), and resets when DueAt moves (TestSweeper_BacksOff). Commit f2f2dac83.
status: addressed
---
