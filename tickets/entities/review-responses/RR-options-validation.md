---
id: RR-options-validation
type: review-response
title: Select/multi-select options validation gaps
finding: |
  The validation schema says options must be "Array of `{value, label}` tuples" but doesn't specify:
  
  1. What if options array is empty? (select with no choices)
  2. What if option values are not unique?
  3. What if option value is empty string?
  4. Maximum number of options?
  5. Can value contain special characters? (used in event.data)
  
  Edge cases to handle:
  - Empty options: Should error at validation time
  - Duplicate values: Should error at validation time  
  - Empty value string: Debatable - could be valid "none" option
  - Max options: Reasonable limit (e.g., 1000) to prevent DoS
severity: minor
status: addressed
resolution: Added options validation table specifying - at least 1 option, max 1000 options, unique values, non-empty value/label strings, no null bytes.
---
