---
id: RR-XPFD04
type: review-response
title: A non-404 anchor error keeps the menu inert, but the docs said only while loading
finding: opening is cleared only when the anchor is gone (401/403/404). A server or network error leaves it set, so the button stays inert until the query recovers. The docs said the button is hidden only while the page's entity loads.
severity: significant
resolution: 'Kept the behaviour: offering types without knowing the links would allow an unlinked create on an entity page. Docs now say the button is busy and does not open until the page''s entity has loaded, which covers the error case. The pages store comment names the error case.'
status: addressed
---
