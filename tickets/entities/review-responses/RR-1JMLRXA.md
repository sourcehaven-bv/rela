---
id: RR-1JMLRXA
type: review-response
title: FlowStep skip is too broad
finding: Skipping change events whenever ctx carries a FlowStep also skips a person's step write for flow A, so flow B on the same subject never sees it.
severity: significant
resolution: A step-completion write carries an explicit step-write marker; it skips only its own run. Outputs and job writes (FlowStep without the marker) stay skipped.
status: addressed
---
