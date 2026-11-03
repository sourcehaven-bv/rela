---
id: RR-LNTYETK
type: review-response
title: sqlite DDL column position
finding: A migrated database appends the column at the end, so the fresh DDL must put it last.
severity: nit
resolution: session_previous goes after session_cause as the last column before UNIQUE.
status: addressed
---
