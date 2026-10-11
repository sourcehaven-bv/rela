---
id: RR-TV67OM
type: review-response
title: postgres names left in sentences where sqlite was neutralised
finding: datamigration/state.go, entitymanager/unique.go mix both styles.
severity: minor
resolution: Neutralised in packages that do not import them.
status: addressed
---
