---
id: RR-2YG33R
type: review-response
title: testifylint swap reversed expected and actual
finding: conflict.Expected is the value under test; swapping it into the expected slot inverts failure messages. testifylint matched the field name.
severity: significant
resolution: Restored the order and bound the field to a local (reported) with a comment, so the rule no longer matches the selector.
status: addressed
---
