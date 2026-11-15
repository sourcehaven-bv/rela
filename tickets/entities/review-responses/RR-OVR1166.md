---
id: RR-OVR1166
type: review-response
title: Past-cap ids were never re-validated by the tracker
finding: 'keepPastCap kept every seen id after the last reported match while a rule stayed truncated. Such an id was never checked again: an entity that stopped and restarted matching past the cap never re-notified, the record grew without limit, and an edited condition kept suppressing notifications.'
severity: significant
resolution: Removed keepPastCap. A rule over MaxMatchesPerRule now reports no matches, and the tracker replaces its record with an untracked marker, so the next run under the cap is silent like a first run. Pinned by TestTracker_TruncatedRuleStartsOver and TestTracker_TruncatedFirstRunWritesNothing.
status: addressed
---
