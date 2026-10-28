---
id: RR-02WXTK
type: review-response
title: comment:read was not part of a marked guard
finding: listOp checked comment:read itself after readTarget, so archplan would not notice if that check were removed.
severity: minor
resolution: The check moved into listGate, which carries the guard marker. openSuggestion's comment:read check still runs inside acceptOp after writeTarget.
status: addressed
---
