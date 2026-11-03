---
id: RR-NYFJ2I
type: review-response
title: 'Precedence of an authored in: is a union in practice; say so'
finding: 'The plan says "an authored in: wins". The load already refuses an authored in: that misses a keyed state (message_in.go:81-88). So an authored in: is always a superset of the derived one, and "wins" can only mean "widens". That is also the whole answer to Q4. Change: state it in Approach 3 and the docs: "an authored in: may add states to the keyed ones; it cannot remove any". The precedence rule then needs no further explanation, and the opt-out in Q4 follows from it.'
severity: minor
resolution: 'Approach 3, AC 3 and the docs now say an authored in: may add states to the keyed ones but cannot remove any, so it works as a union. Q4 is answered from that rule.'
status: addressed
---
