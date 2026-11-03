---
id: RR-O5IFVD
type: review-response
title: Exit codes and diff output
finding: Exit 1 for drift and for errors; diff ordering; routes on + lines; JSON.
severity: minor
resolution: Diff is sorted by section, key, value; + lines carry the granting routes; -o json adds an `expectation` object with the same exit code. Docs state that exit 1 covers both drift and errors.
status: addressed
---
