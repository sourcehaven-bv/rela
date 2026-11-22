---
id: RR-ILPLOOK
type: review-response
title: Action text bodies would be rejected
finding: An action may answer with text/plain, CSV or XML through the shared client; rejecting every non-JSON body broke those actions and the app bridge.
severity: significant
resolution: 'Addressed: only an HTML body (text/html, XHTML, or markup without a content type) is rejected. The action allowlist has no HTML. Tests cover text, CSV and XML action bodies.'
status: addressed
---
