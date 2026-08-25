---
id: RR-7BYA4X
type: review-response
title: 'Sanitize-then-inline ordering is load-bearing: bluemonday strips style attrs, so sanitizing the assembled page yields unstyled mail'
finding: 'The plan states the pipeline as goldmark -> bluemonday -> template -> douceur inline, but never says the ordering is load-bearing. bluemonday''s UGCPolicy strips style attributes outright (AllowStyling() does not restore them), so sanitizing AFTER CSS inlining would destroy every inlined style and produce unstyled mail. Sanitizing the assembled page would additionally strip cellpadding/cellspacing/border/role from the trusted template and drop cid: image sources, breaking the CID-embedded logo.'
severity: critical
resolution: 'Verified empirically against bluemonday v1.0.27 and douceur v0.2.0. Plan now states the invariant explicitly: sanitize the UNTRUSTED CONTENT ONLY, then embed in the trusted template, then inline CSS LAST; never sanitize the assembled page. Confirmed hostile content (script, onerror, javascript: href, style-with-javascript-url) is stripped while template styles inline correctly and mso conditional comments survive. Pinned by acceptance criteria.'
status: addressed
---
