---
id: RR-0G8PI3
type: review-response
title: Hidden source field emptiness leaks before authorization
finding: requireReadableSourceFields refuses only a non-empty hidden value and runs before create/relation/update authorization; requiredFields and the req.Properties key check depend on the same value. A reader without write rights learns via /preflight (unaudited) whether a hidden source field is empty.
severity: significant
resolution: The copy-visibility read now marks every hidden field in Redacted whether or not it holds a value, and the converter refuses on IsRedacted, so the answer is the same for a stored and an empty secret. Pinned by TestTaskItemConvert_RefusesToCopyAnUnreadableField with two sources.
status: addressed
---
