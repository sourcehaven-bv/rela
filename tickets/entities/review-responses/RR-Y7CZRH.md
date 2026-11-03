---
id: RR-Y7CZRH
type: review-response
title: The cost bound covers one tick, not sustained moves
finding: 'Coalescing is per 500 ms tick. While moves keep landing on different visible cards in consecutive ticks, every open board of that type is replayed on every tick, one full list pipeline per principal, plus one ListWaiting that on files reads one directory per anchor (busy.go:73-77). These replays share liveParallel = 4 slots with PUT baselines (live.go:47-48). AC 10 tests a single burst only. Either add a sustained case to the Risks section with numbers (N principals × 100-row board on fs × moves per second), or add a per-subscription minimum spacing. The spacing would defer a selected busy pair to pending without a frame for one or two ticks: that is cheaper than the inbox grid and has no oracle, since pending carries no frame.'
severity: minor
resolution: 'Adopted: Risks now states the sustained bound with numbers (10 principals, 100-card boards on files, a move every tick) and shows it equals today''s graph-write-every-tick worst case. The minimum spacing is deferred as alternative 11 until measurements call for it.'
status: addressed
---
