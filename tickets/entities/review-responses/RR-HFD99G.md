---
id: RR-HFD99G
type: review-response
title: Scanner self-check did not cover the use extractor
finding: The pin only proved that the definition set was populated. A broken var() pattern would find no uses and pass.
severity: significant
resolution: Added a fixture test for both extractors (comments, class modifiers, multi-line var(), fallbacks) and a floor of more than 500 scanned uses.
status: addressed
---
