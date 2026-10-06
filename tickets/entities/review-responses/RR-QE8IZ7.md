---
id: RR-QE8IZ7
type: review-response
title: Debug plus access log removed the request line from the debug stream
finding: With --verbose the request record moved to the access sink, losing the marker that ends a request's SQL lines.
severity: minor
resolution: Under Debug the record goes to both loggers (test LogsToBoth).
status: addressed
---
