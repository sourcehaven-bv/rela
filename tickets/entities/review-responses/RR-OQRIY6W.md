---
id: RR-OQRIY6W
type: review-response
title: Accepting a suggestion could splice A's content into B's view
finding: After awaiting acceptComment, res.content was written into whatever entry was on screen.
severity: minor
resolution: The splice now runs only when servedRef still equals the address the accept went to.
status: addressed
---
