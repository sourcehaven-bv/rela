---
id: RR-J3IFCB1
type: review-response
title: Re-encoding would reformat whole files
finding: Encoding the yaml.Node rewrites formatting of schema.yaml and flows.yaml on the first run.
severity: minor
resolution: 'Plan: cut the automations block by line range and append the encoded triggers block only; crash recovery compares decoded values.'
status: addressed
---
