---
id: RR-TEMYIO
type: review-response
title: Handler built targets twice
finding: Targets were rebuilt without Type for the lookup and Commentable ran twice per row.
severity: minor
resolution: Fixed. Targets are built once with a parallel row index and looked up by targets[j].Key().
status: addressed
---
