---
id: RR-NH1TDM
type: review-response
title: The default identity is "unknown" when $USER is unset (cron, systemd), so every channel write fails
finding: SystemUser falls back to "unknown", which requireCaller refuses, and AssertedBy would then also be "unknown". Make UserText for no_principal tell the user to set $USER or RELA_RUN_AS, and pick a fallback for AssertedBy (LOGNAME or uid).
severity: minor
resolution: UserText for no_principal tells the user to set $USER or RELA_RUN_AS. The asserting OS user falls back to LOGNAME, then the uid.
status: addressed
---
