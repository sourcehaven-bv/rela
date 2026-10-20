---
id: RR-BE1YRL
type: review-response
title: channelSchema reload failure wipes a good descriptor
finding: 'frontend/src/stores/channelSchema.ts:33-43 fetchSchema sets types=[] on any error and then assigns channelTypes. On the first load that is the agreed ''hidden, not broken'' behaviour (R5). On a reload (a refresh event, or checkNames), one transient 502 replaces a descriptor that was working with [], and every open channel panel disappears mid-conversation until the next refresh. Separately, loaded=true after a failed first load means ensureLoaded never retries, so one blip at page load hides channels for the life of the tab unless realtime.yaml changes. Fix: on error keep the held channelTypes, leave loaded false on a failed first load so the next ensureLoaded retries (with a backoff, or on the next refresh), and test both cases.'
severity: significant
resolution: A failed descriptor reload keeps the held types, loaded stays false after a failed first load, and ensureLoaded retries with a capped backoff, with tests for both cases.
status: addressed
---
