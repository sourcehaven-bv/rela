---
id: RR-OB91P8
type: review-response
title: familyExists reported a read error as not found
finding: internal/cli/trace.go turned a backend read error into entity not found.
severity: minor
resolution: familyExists returns the error; requireFamily and the path command report it as a read failure.
status: addressed
---
