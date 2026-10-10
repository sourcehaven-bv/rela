---
id: RR-EKMV8H
type: review-response
title: Error target guessed with strings.Contains
finding: The handler guessed which relation key an error belonged to with strings.Contains on the message.
severity: nit
resolution: Fixed in 3b2b0bdd8. CardinalityError carries the Key, so no string matching is needed.
status: addressed
---

Review finding R2-10.
