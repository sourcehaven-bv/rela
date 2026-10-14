---
id: RR-EHK7W7
type: review-response
title: Markup inside RSS description is lost
finding: textOf flattens embedded well-formed markup though Summary is HTML.
severity: minor
resolution: htmlField serializes child elements for description, content:encoded and Atom html. TestParse_EmbeddedMarkup.
status: addressed
---
