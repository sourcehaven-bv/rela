---
id: RR-G2AYPR
type: review-response
title: getAppHtml interpolates route id into the URL path without encodeURIComponent
finding: 'apps.ts:16 does api.getAxios().get(`/_apps/${id}`) with id from route.params.id (browser URL), not encoded. Server appIDRegex (apps_handler.go:54) 404s anything weird so it''s not a server exploit, but a slash/? in the route param could split the path or inject query params client-side before the server sees it. FIX: encodeURIComponent(id) — defensive idiom for when getAppHtml is reused by a less-trusted caller.'
severity: minor
resolution: getAppHtml now uses encodeURIComponent(id) when building the path (apps.ts).
status: addressed
---
