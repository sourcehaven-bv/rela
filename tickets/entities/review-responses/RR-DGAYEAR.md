---
id: RR-DGAYEAR
type: review-response
title: "A 404 on the entity page was logged as a fault"
finding: "loadView logged every failed load with console.error, including the 404 the page now shows as not-found."
severity: minor
resolution: "A 404 is no longer logged; EntityDetail.notFound.test asserts it."
status: addressed
---
