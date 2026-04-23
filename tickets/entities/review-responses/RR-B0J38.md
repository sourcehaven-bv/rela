---
id: RR-B0J38
type: review-response
title: Empty returnPath produces dangling return_to= on every form link
finding: 'Both document API call sites at internal/dataentry/api_v1.go:2043 and :2060 pass "" for returnPath. The rewriter unconditionally appends return_to= (empty value), producing hrefs like /form/x?return_to=. Frontend then sets returnTo.value = "", the .startsWith(''/'') check fails, and navigation falls through. Noise on every rewritten form link. Pre-existing but worth cleaning up while the rewriter is being touched: if returnPath == "" and no edit-form entity suffix applies, skip the return_to append entirely.'
severity: significant
resolution: rewriteHref now early-returns (base + existingQuery + fragment, unchanged) when returnPath == '' and the link isn't an edit-form (no entity suffix to append). Edit-form links still get return_to=#<entityId> even with an empty returnPath, because the hash fragment is the real payload. Two new subtests pin the behaviour.
status: addressed
---
