---
id: RR-Z2RB0BE
type: review-response
title: Hint must mark RELA_DATAENTRY_USER as local-only
finding: The planned error detail lists $RELA_DATAENTRY_USER as an equal option. On a server bound beyond loopback every caller would then act as that one user, which is worse than the error. GUIDE-server-security describes it as local development only.
severity: minor
resolution: 'Plan updated: the detail and the warning say ''for local use'' for the env var and point to --jwt-* (or --principal-header behind a proxy) otherwise.'
status: addressed
---
