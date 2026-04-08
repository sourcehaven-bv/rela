---
id: RR-UQ5H
type: review-response
title: 'F7: http.Client redirect policy not locked down'
finding: No CheckRedirect set on http.Client. Go's default follows up to 10 redirects. Stdlib does strip Authorization on cross-host redirects since Go 1.8, but same-host redirects preserve the header, and a misconfigured proxy or DNS cache poisoning could still cause surprises. A provider following a 302 to a different path means the request the user authorized and the request the server answered can diverge.
severity: significant
resolution: 'Added CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse } to the http.Client. None of the OpenAI-compat providers we target use redirects in their normal flow, so disabling them is the safer default. New TestProvider_Chat_NoRedirectFollow verifies a 307 Temporary Redirect is rejected rather than followed.'
status: addressed
---
