---
id: RR-7XEXDO
type: review-response
title: No test that the reader's marker passes a quiet tail
finding: 'The badge is right only if opening the channel moves the marker past hidden messages. Today it does: useMarkRead marks up to the newest held seq whatever its display (useMarkRead.ts:56-64), and a hidden row is still held. Nothing in the plan pins this. Without it, a later change that marks "up to the newest shown row" would leave quiet messages above the marker forever: Unread stays 0, but the channel keeps LastSeq > Seq, joins the behind set on every inbox read (inbox.go:681-683) and sorts into the first window group. Add a useMarkRead or deep-link case: the channel''s newest message is hidden, and the mark names that seq.'
severity: minor
resolution: 'ADDRESSED New AC 12: useMarkRead.test.ts case "marks a hidden newest message".'
status: addressed
---
