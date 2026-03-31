---
finding: toString() uses `string(rune('0' + v))` which only works for single-digit integers 0-9. For v=10 you get ':', for v=42 you get 'j'. This will corrupt YAML when integer properties are written via ProjectBuilder.
id: RR-ngqd
resolution: Fixed toString() to use strconv.Itoa(v) for proper integer conversion.
severity: critical
status: addressed
title: 'Bug in toString(): integer conversion broken'
type: review-response
---
