---
id: RR-P7AY0NY
type: review-response
title: Worker pins and AcquireKeyedLock slots are budgeted apart
finding: pin kept two connections free but AcquireKeyedLock admits MaxConns/2 holders outside that count. On the default pool of 4 the two worker pins plus two slot holders took every connection, and the holders' writes waited until their deadline (idproles takes such a lock on IdP role assertions). Found by both reviewers.
severity: significant
resolution: 'One budget: keyedSlotCount is now max(1, (MaxConns-2)/2), leaving room for the two worker pins, and pin admits a lock only while pins + slots + 1 <= MaxConns. Test TestTryKeyedLock_SharesTheBudgetWithKeyedSlots holds both worker locks and the only slot on a pool of 4 and still writes. The conformance pool went from 4 to 6 to keep two slots.'
status: addressed
---
