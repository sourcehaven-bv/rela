---
id: RR-E22B9N
type: review-response
title: A missing base model gave a misleading error
finding: A crash with exit 1 in the base export was read as findings on the base, and the next step failed on a missing model.
severity: minor
resolution: The base step fails unless the model file exists and is not empty.
status: addressed
---
