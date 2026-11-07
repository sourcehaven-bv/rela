---
id: RR-0L4HHOO
type: review-response
title: 'Code: JSON null hidden key never tested'
finding: The three backends test key presence three ways and no seed row held a JSON null under a hidden key.
severity: minor
resolution: Seed row doc/D4 holds secret as null and is expected pending on every backend.
status: addressed
---
