---
id: RR-G2UTG6
type: review-response
title: asserted is not carried through the HTTP wire, OpenAPI author schema or ChannelView.CreatedBy
finding: Section 2 changes only realtime AuthorView. dataentry wireMessage and the v1 author type, openapi channelAuthorRef (user, tool, principal_type), and ChannelView.CreatedBy also need asserted, or the SPA and API clients never see the marker.
severity: minor
resolution: Per Jeroen the marker is audit only, so no wire, OpenAPI or chat field is added.
status: addressed
---
