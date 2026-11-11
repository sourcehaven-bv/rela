---
id: RR-XSBPIVF
type: review-response
title: Some push answers are misclassified or silent
finding: A 400 failed every run; a remote without ETags made updates silent no-ops; a row-level 403 on the refresh GET ends the run as a credential refusal.
severity: minor
resolution: 'A 400 is a held refusal; a missing ETag fails the row with an issue. The refresh GET 403 mapping is the shared pull fetch path and is left as is: the remote answers 403 for a row it hides only when the principal lost access, which is a credential-level problem.'
status: addressed
---
