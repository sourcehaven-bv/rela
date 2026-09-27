---
id: RR-IKY003
type: review-response
title: 'No cross-reference between the guard removal and the SetRemoteMCP JWT refusal'
finding: 'Relaxing the SetRemoteMCP JWT refusal would silently leave the endpoint without the SDK Host guard; the CSRF exemption is cross-referenced there but the guard was not.'
severity: minor
resolution: 'Fixed. The SetRemoteMCP doc now names the disabled guard as a second thing to revisit if the refusal is relaxed.'
status: addressed
---
