---
id: RR-XPFB02
type: review-response
title: The Create menu shows every type while an entity page's anchor loads
finding: 'PageView sets pageStore.current only after the anchor entity has loaded, because a link must name the face on screen. Until then offered() saw no entity page and returned every create: type. On an uncached anchor the menu briefly offered unlinkable types, and a row opened in that window got no link (target() was undefined), so the new entity was created unlinked on an entity page.'
severity: significant
resolution: 'Added pageStore.opening, set by PageView while an anchor id is in the URL and current is not set, unless the anchor is gone (hidden or missing). offered() returns nothing while it is set, and the Create button is shown busy (RlButton loading: aria-busy, aria-disabled, activation blocked) until the link is known. PageView clears it on unmount. A hidden or missing anchor leaves opening false, so the not-found page keeps the full menu as before. Tests in PageView.test.ts and SpaceCreateMenu.test.ts; the menu test fails without the offered() guard. Docs updated.'
status: addressed
---
