---
id: RR-Q4FH2X
type: review-response
title: useSessionDecisions is a 15-callback bag that no unit test uses
finding: 'useSessionDecisions.ts:23-55: DecisionDeps passes back almost every OpenChannel internal: ch, post, setPending, submitting, editing, discardEdit, selectInComposer, setFormError, handle, confirm, toast, focusRow, focusHeader, messageType, channelType. Only the panel-level ChannelsPanel.test.ts exercises it, so the seam buys no testability. It only moves about 120 lines out of OpenChannel, with a wide coupling surface (CLAUDE.md asks for narrow consumer interfaces of one to three methods). Either unit-test the composable against fakes, which is where the confirm and generation race above would be cheap to pin, or fold it back into OpenChannel. As written it is extraction for line count.'
severity: minor
resolution: 'Kept the composable but grouped its dependencies into DecisionComposer and DecisionUI interfaces, moved the composer-select logic into it, and added useSessionDecisions.test.ts with fakes (12 tests: generation capture, pending timing, double click, channel switch, refusals, form opening, focus).'
status: addressed
---
