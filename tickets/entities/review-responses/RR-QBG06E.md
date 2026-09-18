---
id: RR-QBG06E
type: review-response
title: DocumentsPanel's tab selector stays live after a denial, offering a retry that will fail
finding: The panel wraps its chrome in v-if="availableDocuments.length > 0", computed from config rather than content. After a denial blanks docContent the tab selector and Refresh button remain, so the user faces an empty state with controls that fire another denied render and another toast. DocumentView has no equivalent — its empty state is terminal — so a reader comparing the two files would wrongly assume they behave alike.
severity: significant
resolution: 'Documented rather than changed, with a comment at the panel''s empty state. Hiding the chrome would misreport a configured document as unconfigured, and the panel is one section of an entity page rather than a whole route, so leaving the surrounding controls is the consistent behaviour. Not a confidentiality issue: config names are not secret (docs/acl-security.md), which the component already notes where it builds availableDocuments.'
status: addressed
---

The reviewer flagged this as a UX dead end rather than a leak, which is the
correct reading. The comment makes the divergence from DocumentView deliberate
and explains it, so the next person does not "fix" one to match the other.
