---
id: RR-9VVFON
type: review-response
title: Link path keeps publisher bytes
finding: url.Parse keeps the original path in RawPath, so item.link could carry ![](...) after a ) and place an image in [t]({{item.link}}).
severity: critical
resolution: link() re-encodes the escaped path with linkEscaper; test case added.
status: addressed
---
