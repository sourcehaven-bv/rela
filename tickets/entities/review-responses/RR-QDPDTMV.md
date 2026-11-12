---
id: RR-QDPDTMV
type: review-response
title: No hop guard against job writes that start runs
finding: A script job's writes could start other runs whose jobs write again; automationjobs carries a hop count for this.
severity: significant
reason: No write can start a run until TKT-GYXOTQC adds write-driven start triggers. A job write therefore cannot loop back into a new run today; the hop guard is recorded on TKT-GYXOTQC, where that loop becomes possible.
status: deferred
---
