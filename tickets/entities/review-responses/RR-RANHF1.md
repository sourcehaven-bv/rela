---
id: RR-RANHF1
type: review-response
title: No redirect when the successor has no face in the requested world
finding: The successor is resolved in the request's world. When the world ranks faces and the successor has no face in it, no redirect happens and the reader gets the old id's miss, even though the successor exists and is readable in another world.
severity: significant
reason: 'Deliberate: the redirect follows the same world as the request, exactly as mention resolution does for the @-mention chip of the same old id. Resolving in another world would make the redirect and the chip disagree, and send the reader to a page that answers world-absent for the successor anyway. A reader who switches to a world where the successor has a face gets the redirect. Resolving through ''any readable face'' would also need a new resolver mode that bypasses world ranking, which is out of scope for this ticket.'
status: wont-fix
---
