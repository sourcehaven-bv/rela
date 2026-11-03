---
id: RR-MH6PKL
type: review-response
title: decisionTone contract is ambiguous for a button that opens the composer and can change today's look
finding: 'Today the derived rule applies only to a one-click button: channelView.ts:445-446 calls endTone only when !opensForm, so a form-opening type button into a cancelled state is not red. The plan says one helper applies the options and that style applies to a form-opening button too (plan lines 386-389, AC 6). If channelView calls decisionTone for every button to get style, the derived danger now also reaches form buttons, which changes the look of files with no keys and breaks AC 5. Change the plan to state the rule exactly: on a form-opening button only an authored style applies and the derived part contributes nothing; on a one-click button the authored keys override the derived ones per key. Simplest shape is two functions, derivedTone(targets, stateOf) (today''s endTone, unchanged) and applyOptions(view, options, oneClick), so the derived rule is untouched code. Add a case "form button into a cancelled state stays plain without options".'
severity: significant
resolution: endTone stays unchanged and a new applyOptions applies authored keys per key on a one-click button; on a button that opens the composer only an authored style applies. Added the case "form button into a cancelled state stays plain without options".
status: addressed
---
