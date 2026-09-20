---
id: RR-CARPTY
type: review-response
title: 'CarriesProperty re-admits the empty list the validator exists to refuse'
finding: "config.go's CarriesProperty returns true for `d == nil || len(d.Properties) == 0`, but validateEntityDuplicate refuses a non-nil-but-empty Properties at load with a comment stating there is exactly one spelling of the default. The method adds a second spelling for a state the validator makes unreachable. A reader cannot tell which invariant actually holds. The TS mirror has the same shape but no validator behind it, where it IS load-bearing — which makes the Go copy look intentional when it is redundant. TestDuplicateConfig_CarriesProperty omits the empty-slice-on-non-nil case entirely."
severity: significant
resolution: CarriesProperty now treats only the nil receiver as carry-all; an empty allowlist carries nothing, matching what the validator makes unreachable. The TS mirror aligned. Both cases pinned.
status: addressed
---

## Suggested resolution

Drop the len==0 arm on a non-nil receiver (nil still means carry-all), or document why the unreachable state is handled anyway. Cover the case in the test either way.
