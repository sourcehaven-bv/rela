---
id: RR-LK1YGE
type: review-response
title: api-reference live-views paragraph misstates the fallback rule for the inbox and for busy lists
finding: 'docs/data-entry/api-reference.md:679-683 says a busy list and an inbox path get no frame when a round started by a channel change skips them. Two parts are wrong. (1) An inbox path never gets a fallback frame, in any round (live.go fallback: !s.inbox). A full round started by an entity change does not frame it either. (2) The busy rule follows the round type (only != nil), not what started the round. Topic rounds are also started by the safety turn, by a presence change, and by pending replays alone (live.go:697), and none of these is a channel change. Full rounds are also started by lease changes and realtime.yaml reloads, not only by entity changes. Clients and the next maintainer will rely on this contract. Fix: say that an inbox path never gets a frame without a check. Say that a list with busy gets none in a round that is not a full re-check (rounds started by channel changes, presence changes, the periodic safety check or skipped-pair retries). Say that in a full round (an entity or lease change, or a reload) it gets the frame like any list.'
severity: significant
resolution: 'Fixed: the Live views paragraph now lists three exceptions: an inbox path never gets an unchecked frame; a busy list gets none in a round that is not a full re-check (channel changes, presence changes, safety check, retry of skipped pairs) and gets it in a full re-check (entity change, lease change, reload); and a pair a topic round took only as pending gets none (finding 20).'
status: addressed
---
