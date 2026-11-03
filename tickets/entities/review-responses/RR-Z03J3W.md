---
id: RR-Z03J3W
type: review-response
title: Slot index Next breaks when a reload lengthens every or first
finding: 'Plan Approach 5 (lines 551-555) stores Next, a slot index, and derives the slot as since + first + Next*every from the current config. The rejected alternative at lines 718-719 claims this survives an every change, but it only survives a shorter every. A longer one pushes the next reminder far out. Example: every 1m to 30m with Next=100 puts the next slot at since + 50h, so the reminder goes silent for two days. Changing first shifts every slot in the same way. Plan change: store Last (the time of the last post, zero for none) instead of Next. The next slot is the first grid point since + first + k*every that is strictly after Last, or slot 0 when Last is zero. Missed slots are still skipped and the phase stays on the grid. A config change in either direction gives at most one every of delay. The CAS becomes (Since, Last). Resume sets {Since: at, Last: zero, Count kept}. Update the rejected-alternative text, TestNextSlotAfter, testSessionRemindersStored (a negative Next becomes a Last after the fire time) and TestTimers_RemindReload, and add a row that lengthens every and one that changes first.'
severity: significant
resolution: 'Addressed: records store Last, not a slot index. The next slot is the first grid point after Last; CAS on (Since, Last); resume sets Last zero. Reload rows for longer every and changed first added (AC 10, AC 12).'
status: addressed
---
