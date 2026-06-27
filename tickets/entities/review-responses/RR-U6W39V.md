---
id: RR-U6W39V
type: review-response
title: csp_origins is never validated — CSP-directive injection via config
finding: 'The design said csp_origins is ''validated as origins at load'' but validateApps (validate.go:929-947) only checks the app id + File path; CSPOrigins flows unvalidated from YAML into buildAppCSP (apps.go:37), which does strings.Join(cspOrigins, '' '') and concatenates into both the CSP header and the injected <meta>. A csp_origins value containing a space/semicolon injects arbitrary CSP directives, e.g. "''unsafe-eval''; script-src * ''unsafe-inline''" neuters the baseline policy. html.EscapeString (apps.go:104) only escapes <>&''" — NOT spaces/semicolons (the CSP delimiters), so escaping doesn''t help. Requires control of data-entry.yaml (trusted), but the design promised validation a reviewer/operator will assume exists, and csp_origins is exactly the field a non-expert hand-edits. FIX: validate each origin at config load via url.Parse + scheme/host check, rejecting anything with whitespace, '';'', '','', or quotes (reuse middleware_security.go normaliseOrigin). Confirmed by reading validate.go + apps.go.'
severity: critical
resolution: Implemented csp_origins validation in validateApps (validate.go). New validateCSPOrigin rejects any origin containing whitespace/';'/','/quotes (cspOriginForbidden regex) AND requires a parseable http(s) scheme://host with no path/query/fragment/userinfo. Added TestValidateApps with cases for injected-directive, semicolon, path, and missing-scheme origins — all now rejected at config load. The injection primitive is closed.
status: addressed
---
