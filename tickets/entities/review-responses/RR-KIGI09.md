---
id: RR-KIGI09
type: review-response
title: State godoc claims forward compatibility that does not hold
finding: The State godoc justifies the three-map layout partly as letting an older binary read a newer file. An older binary can read it, but saveState marshals the whole struct, so its first write drops failures and next_retry. A downgrade or mixed-version rollout resets every in-flight ladder and un-suppresses the schedule. A map-of-structs layout would behave identically, so the stated justification is wrong. Backward compatibility is real and tested; only the forward-compat claim is false.
severity: significant
resolution: 'Corrected the State godoc: it now claims backward compatibility only, states explicitly that saveState drops unknown fields so a downgrade resets in-flight ladders, notes a map-of-structs layout would behave identically, and documents that absent and empty are equivalent for the omitempty retry maps.'
status: addressed
---
