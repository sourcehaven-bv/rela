---
id: RR-7R3R2HY
type: review-response
title: Lines joined across missing buckets
finding: A straight segment across an empty bucket showed values that do not exist.
severity: minor
resolution: segments() breaks a line where the next bucket is missing; isolated points get a dot. Unit and component tests added.
status: addressed
---
