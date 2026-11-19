---
id: RR-71HARYS
type: review-response
title: UrlWidget has a fragment root so class and attrs fall through nowhere
finding: The template had several root nodes, so a class or attribute set by the parent did not land on the widget.
severity: minor
resolution: The template has one root per branch (v-if/v-else-if/v-else). A unit test checks that a class lands on the root.
status: addressed
---
