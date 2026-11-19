---
id: RR-I53AMUX
type: review-response
title: mailto query can add hidden recipients
finding: 'A mailto: value with ?bcc= or ?cc= adds recipients that the reader does not see in the link text.'
severity: minor
resolution: 'linkHref drops the query and fragment of a mailto: link. A unit test covers it.'
status: addressed
---
