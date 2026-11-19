---
id: RR-58G9BVS
type: review-response
title: Config exceeds the plimsoll field cap
finding: The new ListButtons field makes Config 32 exported fields against a pin of 31.
severity: critical
resolution: Raised the pin to 32 and named list_buttons in the format-mirror comment, as that comment prescribes for a new top-level key.
status: addressed
---
