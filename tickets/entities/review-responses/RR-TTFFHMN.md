---
id: RR-TTFFHMN
type: review-response
title: IdP suspend over an API suspension let an API resume lift the IdP hold
finding: When the IdP suspended a tenant already suspended by api the cause stayed api; a later API resume then lifted the IdP suspension (fail open).
severity: significant
resolution: 'The IdP now takes the cause over from api. A new api_hold column keeps the API''s own hold: an IdP resume leaves the tenant suspended by api and an API resume is refused while the IdP holds it. Tested in TestProvisioner_IDPAndAPIHolds.'
status: addressed
---
