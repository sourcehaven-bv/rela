---
id: RR-55WMF2Q
type: review-response
title: Rows carry edit data the page may not use
finding: withRowEditData sends all visible properties and verdicts, also above the 100-row cap and for properties no column shows.
severity: minor
reason: No extra store reads (budget test). Trimming to column properties and moving the cap to the server is a payload optimisation for later.
status: deferred
---
