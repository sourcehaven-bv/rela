---
id: RR-TKPCIA
type: review-response
title: Read path must exclude typing signals from Signals, in one shared split
finding: 'Step 8 splits sigs in Head only. Read (internal/realtime/read.go:139-150) also calls liveSignals and renders every signal through r.signal into ReadView.Signals. Step 9 adds TypingNow but does not say that typing signals leave Signals. If they stay, MCP read_channel shows them as {type:"", props:null}, and the caller''s own signal is not excluded. Plan change: one helper (for example splitSignals(c, sigs) (operator []signal, typing []TypingView)) used by both Head and Read. Add a TestReadChannel_Typing assertion that signals contains no typing entry and that self-exclusion holds on the MCP path.'
severity: minor
resolution: 'Plan updated: one helper Service.liveSignals(c) returns operator signals and typing views for both Head and Read; Signals holds operator signals only; TestReadChannel_Typing asserts both and self-exclusion.'
status: addressed
---
