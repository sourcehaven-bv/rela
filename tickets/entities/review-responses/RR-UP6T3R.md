---
id: RR-UP6T3R
type: review-response
title: SidePanel lists and links peers by bare id in the default world
finding: SidePanel fetches /_sidepanel without ?world= and its edit links use entity.id, a bare id. A faced peer that only the page's world serves is missing, and a write to a bare faced id is refused.
severity: significant
reason: Pre-existing and needs a server change (world-aware /_sidepanel and addresses in its payload). This fix only adds the world to the link, which makes the form read the face the page's world serves instead of the default world's. Recorded as a follow-up on the bug.
status: deferred
---
