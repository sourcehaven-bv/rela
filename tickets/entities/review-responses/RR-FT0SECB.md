---
id: RR-FT0SECB
type: review-response
title: Client range grammar was stricter than the server
finding: ParseRetention accepts +5d, which the chart could not read, so it showed every row.
severity: nit
resolution: Config load also requires digits and one unit; client test covers +5d.
status: addressed
---
