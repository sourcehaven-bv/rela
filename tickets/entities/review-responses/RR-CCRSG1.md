---
id: RR-CCRSG1
type: review-response
title: The SPA drops `read` on every follow, and every mark costs three GETs
finding: 'useChannel.follow calls setChannel(page.channel). That replaces head.channel with the message page''s channel, which by decision 5 has no `read`. After every incoming message the held marker is therefore undefined, so the mark-read logic compares against nothing and sends another PUT. A mark also raises no rev, so its own live frame brings no messages. follow then takes the "moved the head itself" branch: one messages?after_rev GET, a refetchHead, and a readNewest. That is three requests per mark per tab. Fix: setChannel keeps `read` from the held head. useMarkRead takes the PUT answer as the held marker. If the broadcast stays, the head frame for a mark-only change should not trigger readNewest. Add useChannel tests: read survives a follow; one mark causes at most one extra request.'
severity: significant
resolution: setChannel keeps read from the held head; PUT answer is the held marker; a head frame with equal rev and last_seq only updates read and fetches nothing. useChannel tests for both.
status: addressed
---
