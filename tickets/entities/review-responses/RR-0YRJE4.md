---
id: RR-0YRJE4
type: review-response
title: Coverage exclusion ^tools/seqtrace$ never matched
finding: .testcoverage.yml matches file paths, so the $ anchor excluded nothing; the runtime package fell below the 50% floor.
severity: significant
resolution: Changed to ^tools/seqtrace/[^/]+\.go$.
status: addressed
---
