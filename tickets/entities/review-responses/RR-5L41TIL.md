---
id: RR-5L41TIL
type: review-response
title: Mail job rendered as run_as would mail run_as's view to the recipient
finding: Scheduled mail renders under the recipient's principal; a flow job runs as run_as, so RunScheduledTemplate would send whatever run_as may read.
severity: critical
resolution: 'kind: mail is out of scope; it moves to TKT-VVDY2RM, which must render as the recipient''s principal. Only kind: webhook ships.'
status: addressed
---
