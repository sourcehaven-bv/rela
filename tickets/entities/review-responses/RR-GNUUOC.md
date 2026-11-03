---
id: RR-GNUUOC
type: review-response
title: Watch limits and handler must not grow Server
finding: 'Architecture design review: Server is at 25 of 27 methods; limits must be per Server, not per HTTPHandler; tests that build Server literals would lack them.'
severity: minor
resolution: An unexported watchHandler built in NewServer owns limits, map and key, registered through a closure; watch tests go through NewServer.
status: addressed
---
