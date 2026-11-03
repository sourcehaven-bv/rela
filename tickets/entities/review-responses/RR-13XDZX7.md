---
id: RR-13XDZX7
type: review-response
title: Plimsoll caps break with the planned methods
finding: call is at 40 methods (authz.go:35), Service at its directive 42, both channel stores at max-exported-methods 34; call.decisionAffordances, a Service method and GetMessages would exceed them.
severity: significant
resolution: decisionAffordances, WaitingMessages and messageRenderer are package functions; the stores' max-exported-methods directive goes from 34 to 35 under the required-interface exception, helpers as package functions.
status: addressed
---
