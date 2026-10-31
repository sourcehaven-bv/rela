---
id: RR-S9CWDQT
type: review-response
title: Check at send time; floors for rare paths
finding: Check criterion 2 at send time; give MAX_CONFLICT_ATTEMPTS a floor or pin it with Gate.
severity: minor
resolution: Criterion 2 is checked when the request reaches the server; a tab's saves are serialized, so nothing it is shown arrives between send and arrival. Floors cover accepted writes, 412s, conflicts, merged writes, faults and saves out of attempts.
status: addressed
---
