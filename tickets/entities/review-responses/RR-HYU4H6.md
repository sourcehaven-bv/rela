---
id: RR-HYU4H6
type: review-response
title: Client tokens with no matching baseline keep config:edit
finding: The ceiling refuses config:edit only when a baseline matches the principal_type, so a 'pat' or 'service' token with no baseline was unrestricted.
severity: significant
resolution: mayConfigure admits only an empty or 'user' principal_type. Gate test cases for both.
status: addressed
---
