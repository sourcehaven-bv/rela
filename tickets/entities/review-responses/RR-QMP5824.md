---
id: RR-QMP5824
type: review-response
title: A members object instead of a list clears roles silently
finding: rela.json.decode turns an object and a list into a table, so a members object passes the type check and ipairs finds nothing.
severity: minor
resolution: Kept the clear (safe direction) and documented it in a comment; pinned by the "members is an object" case.
status: addressed
---
