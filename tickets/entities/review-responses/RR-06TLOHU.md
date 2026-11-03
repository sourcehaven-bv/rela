---
id: RR-06TLOHU
type: review-response
title: Two adjacent doc rows were already wrong
finding: The --where table says = with glob works on all types and the validation table uses priority>low. Both predate this change.
severity: nit
reason: Pre-existing and unrelated to the regex fix. Kept out of this commit to keep it focused.
status: wont-fix
---
