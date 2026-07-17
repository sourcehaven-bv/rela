---
id: RR-1TBFJS
type: review-response
title: Datetime validation type-error message misleading (accepts time.Time too)
finding: 'validation.go datetime arm default case returns ''Must be a datetime string'' but the arm also accepts time.Time. For the int-rejected case the message implies only strings are valid. Fix: reword to ''Must be a datetime string or timestamp''.'
severity: nit
resolution: Fixed. The datetime validation arm's default-case error message is now 'Must be a datetime string or timestamp' (was 'Must be a datetime string'), reflecting that the arm accepts both string and time.Time.
status: addressed
---
