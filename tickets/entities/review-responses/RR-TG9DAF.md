---
id: RR-TG9DAF
type: review-response
title: Other audit and attribution sinks drop AssertedBy; rela scheduler ignores --run-as
finding: history_purge.go records only PrincipalUser and PrincipalTool, and store.Attribution (entitymanager/attribution.go) keeps User and Tool only, so pg/sqlite version history shows system:cron with no marker. scheduler.stampTaskAuditContext restamps per task, so a global --run-as is silently ignored there. Decide per sink, and refuse --run-as for the scheduler command.
severity: minor
resolution: The marker goes to the graph audit only; version history keeps user and tool, documented. rela scheduler refuses --run-as.
status: addressed
---
