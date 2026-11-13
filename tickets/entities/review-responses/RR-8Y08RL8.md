---
id: RR-8Y08RL8
type: review-response
title: Retention and leg dates could unbalance a ledger
finding: A ledger could declare retention, and the legs of one transaction could carry different ts, so Expire could drop one leg and free the group key. Reverse negated whatever legs remained without re-checking the balance.
severity: significant
resolution: The schema refuses retention on a ledger; all legs of a transaction must share one ts (TestLedger_LegsShareTS); Reverse re-checks the balance before it appends.
status: addressed
---
