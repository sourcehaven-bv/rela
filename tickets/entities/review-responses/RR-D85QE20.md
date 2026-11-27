---
id: RR-D85QE20
type: review-response
title: check outputs on relation rules were only logged
finding: A check on a relation rule did not block the write.
severity: significant
resolution: An error check refuses the write\; a warning check or unparseable where on a relation rule is a load error.
status: addressed
---
