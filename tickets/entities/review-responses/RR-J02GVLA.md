---
id: RR-J02GVLA
type: review-response
title: The world is dropped from the job
finding: The background run read in the default world, not the world the user queued it from, so a faced address could resolve differently.
severity: significant
resolution: The job payload carries the request's world; bindBackgroundRequest binds it again with the read grant check, and an unknown world fails without a retry.
status: addressed
---
