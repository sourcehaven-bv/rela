---
id: RR-number-parsing
type: review-response
title: Number field parsing and edge cases unspecified
finding: |
  The plan specifies number fields return `number` type but doesn't address:
  
  1. What about non-integer input when step is 1? (e.g., "3.5")
  2. What about values outside min/max range?
  3. What about empty input for non-required number field? (nil? 0? error?)
  4. What about scientific notation? (1e10)
  5. What about locale-specific decimal separators? (1,5 vs 1.5)
  6. NaN and Infinity handling?
  
  Recommendation: 
  - Parse as float64, return as Lua number
  - Enforce min/max at transport layer (huh supports this)
  - Empty non-required → nil
  - Reject NaN/Infinity at validation
  - Use "." as decimal separator (standard)
severity: minor
status: addressed
resolution: Added implementation notes for number field - parse as float64, enforce min/max in validator, empty non-required returns nil, reject NaN/Infinity, use "." decimal separator.
---
