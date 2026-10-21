---
id: RR-68GHD9
type: review-response
title: Converting drops inline content of the item line
finding: link replaces the paragraph content and remove deletes it; links and existing references on the line are lost (taskItemText flattens). The remove doc claims nothing is dropped.
severity: significant
resolution: Link mode now replaces only plain text with the reference and keeps linked text, references and images after it. Remove mode links instead when the line holds a link, image or other inline node the item text cannot carry. The guide documents both. Editor tests cover each case.
status: addressed
---
