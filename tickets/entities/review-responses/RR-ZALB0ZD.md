---
id: RR-ZALB0ZD
type: review-response
title: filerecords Replace retry duplicates a row
finding: A failure between the two file writes left the row twice and a retry added a third copy.
severity: minor
resolution: Replace removes any copy of the row from the target file before appending.
status: addressed
---
