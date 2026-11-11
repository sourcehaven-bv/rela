---
id: RR-UJSP1QI
type: review-response
title: Scope key could be ambiguous
finding: The scope was base:user; a colon in either part could in theory make two pairs share a key.
severity: nit
resolution: notificationScope is now JSON.stringify([base, user]).
status: addressed
---
