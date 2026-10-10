---
id: RR-I5ZWME
type: review-response
title: Guide relies on GET / succeeding through the proxy
finding: If a proxy redirects GET /, restish does not see the Link header and discovery fails; the old spec_files setup did not depend on /.
severity: minor
resolution: The guide keeps a one-line spec_files fallback for proxies that redirect GET /.
status: addressed
---
