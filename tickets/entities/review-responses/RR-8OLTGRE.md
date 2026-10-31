---
id: RR-8OLTGRE
type: review-response
title: primePage passes nil rows to PrimeTraversals
finding: 'Code review: primePage filters nils for the ids but passes the original rows to PrimeTraversals, which reaches primeVerdicts without a nil check.'
severity: minor
resolution: primePage builds one slice of non-nil rows and passes it to both primes.
status: addressed
---
