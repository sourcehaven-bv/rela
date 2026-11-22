---
id: RR-58JSBT7
type: review-response
title: A blank link_text passed validation
finding: link_text with only spaces loaded and showed an empty link.
severity: minor
resolution: The loader rejects a blank link_text and link_text without link; display_test.go covers both.
status: addressed
---
