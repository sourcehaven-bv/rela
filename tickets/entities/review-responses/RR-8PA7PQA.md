---
id: RR-8PA7PQA
type: review-response
title: rela flow show -o json prints a partial list on a broken flows.yaml
finding: With no name, show wrote a JSON list of only the headless flows and then returned the flows.yaml error.
severity: minor
resolution: In JSON mode show now writes nothing and returns the error. Text mode still lists the headless flows before the error.
status: addressed
---
