---
id: RR-VYPVQNM
type: review-response
title: Exit code 1 could mean a build failure
finding: set -e made go build and cp failures exit 1, which the workflow reads as a broken limit; failed benchmarks showed as Not run.
severity: minor
resolution: 'Addressed: builds and cp exit 2; a benchmark failure passes -bench-note so the report says the benchmarks failed.'
status: addressed
---
