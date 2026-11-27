---
id: RR-33HT6KJ
type: review-response
title: Dead return after t.Fatalf in waitForEvent
finding: return nil after t.Fatalf is unreachable; the for loop is a terminating statement.
severity: nit
resolution: Removed.
status: addressed
---
