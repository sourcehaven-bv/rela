---
id: RR-IV3EXE
type: review-response
title: visibleTitles detects the batch resolver by type assertion
finding: views_handler.visibleTitles asserts viewReader to idResolver and keeps a world-first fallback that production never takes. Also raised as a nit by the security review.
severity: minor
reason: Removing it means retyping viewReader (visibility.Reader across App and its tests) to a consumer-side interface; visibleReader cannot replace it because it does not redact. The fallback fails closed. Belongs with the view traversal work in TKT-H57VZJ.
status: deferred
---
