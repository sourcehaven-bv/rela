---
id: RR-M4AD2
type: review-response
title: internal/dataentry floor at 60% has only 1.5pp headroom to actual 61.5%
finding: .testcoverage.yml sets dataentry=60 but measured coverage is 61.51%. One untested handler removal trips CI. Either lower to ~55 for headroom or tighten tests. Reviewer also flagged entity (85/87.1) and project (85/87.3) as tight at ~2pp.
severity: critical
resolution: '.testcoverage.yml floors lowered to give ~5pp uniform headroom: dataentry 60→55, entity 85→80, project 85→80. Local `just coverage-check` still passes (71.8% total vs 65% floor).'
status: addressed
---
