---
id: RR-TN64ISM
type: review-response
title: OAuth token refresh bypasses the HTTP seam
finding: buildTokens did not pass the WithHTTPClient client to the token broker.
severity: minor
resolution: buildTokens passes the seam client as BrokerDeps.HTTP when set.
status: addressed
---
