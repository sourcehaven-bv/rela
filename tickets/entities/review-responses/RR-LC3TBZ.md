---
id: RR-LC3TBZ
type: review-response
title: Unused-key check cannot see dynamically built keys
finding: The unused-key check looks for each key literally in src/. A key built at runtime (language.${code}) would be reported unused or hide a missing key.
severity: significant
resolution: 'Plan: every key is written literally; maps from code to key are literal records (apiErrors.ts, language names). The guard test documents the rule.'
status: addressed
---
