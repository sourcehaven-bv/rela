---
id: RR-RFRS5L
type: review-response
title: Markdown [text](/entity/ID) links cause full-page reload
finding: Anchors rendered via v-html hard-navigate (no SPA click delegation), so entity links in headers reload the page. Consistent with existing EntityDetail behavior (not a regression); docs recommend this pattern. Won't-fix for this ticket — SPA-routing v-html links is a separate cross-cutting concern.
severity: nit
reason: Not a regression — v-html anchors hard-navigate everywhere in the app (EntityDetail included); SPA click-delegation for rendered markdown links is a separate cross-cutting concern, out of scope for this ticket. Standard entity links still work, just with a reload.
status: wont-fix
---
