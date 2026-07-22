---
id: RR-9NTWBJ
type: review-response
title: CLI swallowed the specific 'no Chrome' error
finding: docscapture.New() returns a precise error ('no Chrome/Chromium browser found on PATH'), but the CLI discarded capErr and left Capturer nil, so a screenshot manual failed with the generic 'no capturer configured' instead of the actionable message.
severity: significant
resolution: 'The CLI now stashes capErr.Error() into Options.CapturerErr; the screenshot resolver surfaces it in its fail-loud message when the capturer is nil, so the operator gets the specific Chrome+PATH reason. Note: also handles the SPA-not-built reason (standUp''s CheckEmbeddedSPA).'
status: addressed
---
