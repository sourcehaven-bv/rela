---
id: RR-90QE1O
type: review-response
title: 'Integer values are not raw: they round-trip through Number'
finding: 'RlChannelField.vue:56-68. numberValue parses the text with Number() and setNumber writes String(n) back, so what the composer emits has already passed through a float. Examples: "1e3" becomes "1000", "007" becomes "7", and a 20-digit id becomes "12345678901234567000". The README and the header say values leave raw and the app owns conversion (R20, refuse outside the safe range). The app then validates a value the user never typed, and its error message shows a different number. Fix: emit the input''s raw text for integer fields. Either use RlTextField with inputmode=numeric, or have RlNumberField expose the raw string. Leave parsing and range checks to channelValues.ts.'
severity: minor
resolution: Integer fields are now an RlTextField with inputmode numeric and emit the raw text the user typed (story IntegerIsRaw).
status: addressed
---
