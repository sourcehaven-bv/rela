---
id: RR-IFDCZY
type: review-response
title: Collision pre-check runs outside the row transaction
finding: A user creating the destination face between the pre-check and the thread move can still receive the thread.
severity: minor
resolution: 'Documented as an accepted residual on moveThreads: the window is one read, and the in-Tx check then refuses the row move with the id named.'
status: addressed
---
