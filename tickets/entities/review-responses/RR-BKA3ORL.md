---
id: RR-BKA3ORL
type: review-response
title: Unset on absent value is a data-loss trap
finding: A filtered export or a path typo would wipe the property on every matched entity.
severity: significant
resolution: 'An absent value leaves the stored value alone. Clearing is opt-in per property with clear_missing: true and is listed as a separate change kind in the plan.'
status: addressed
---
