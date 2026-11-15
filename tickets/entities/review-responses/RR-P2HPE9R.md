---
id: RR-P2HPE9R
type: review-response
title: Hook print corrupted -o json
finding: Base print wrote to stdout; globals persist across records.
severity: minor
resolution: print writes to stderr. The guide says one Lua state serves every record of a mapping.
status: addressed
---
