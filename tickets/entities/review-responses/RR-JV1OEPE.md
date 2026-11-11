---
id: RR-JV1OEPE
type: review-response
title: Editor 404 tested by direct call, headless-only
finding: The test bypassed the route and ran only under the tag.
severity: minor
resolution: Replaced with untagged TestAppEditorAssets_NotEmbedded404 that stubs the sources and requests the reserved app paths.
status: addressed
---
