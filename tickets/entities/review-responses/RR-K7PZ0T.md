---
id: RR-K7PZ0T
type: review-response
title: Unvalidated BaseURL defeated the href scheme allowlist
finding: 'safeHref carefully rejected a javascript: LINK, then for a relative link concatenated it onto Options.BaseURL, which nothing validated. Verified: BaseURL ''javascript:alert(1)//'' with link ''/x'' produced href="javascript:alert(1)/x" in the HTML part and the same string in the text part. mail.Config.Validate does check base_url, but that is a different package validating a different struct with nothing wiring them together — and mailrender is documented as a standalone leaf that future callers (the Lua transport, a scheduler digest) construct Options for directly.'
severity: critical
resolution: mailrender.New now validates BaseURL against the same http/https allowlist safeHref applies, and rejects control characters in LogoCID. Closing it at the boundary means it holds for every caller rather than depending on each one having validated first. Pinned by TestNew_RejectsHostileBaseURL covering javascript:, data:, scheme-relative //evil and a bare hostname, plus the accepted forms.
status: addressed
---
