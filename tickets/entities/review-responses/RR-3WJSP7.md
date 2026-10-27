---
id: RR-3WJSP7
type: review-response
title: pg and sqlite differ on a non-integral numeric text
finding: pg returned no value for a stored 5.0 because numeric::text is 5.0.
severity: nit
resolution: integralSQL returns trunc(n), so the text is integral.
status: addressed
---
