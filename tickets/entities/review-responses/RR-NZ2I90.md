---
id: RR-NZ2I90
type: review-response
title: Use Timed bool (zero-value=all-day), not AllDay bool — default footgun
finding: 'Plan adds `AllDay bool` to calfeed.Event. Zero-value would be AllDay=false=timed, so every existing Event literal (~20 across ical_test.go/json_test.go + feed_provider.go:223) that sets Start but not the flag would silently become TIMED and emit DTSTART:...T000000Z instead of DTSTART;VALUE=DATE. It would also change ETags of all existing all-day feeds (breaking subscriber caches). Fix: use `Timed bool` instead — zero-value=all-day=safe/backward-compatible default; no existing construction site needs editing; ETags preserved. feed_provider sets Timed=(dateDef.Type==PropertyTypeDatetime).'
severity: critical
resolution: Adopted. Use `Timed bool` on calfeed.Event (zero-value = all-day). Renderers branch on `Timed`; feed_provider sets `ev.Timed = (dateDef.Type == metamodel.PropertyTypeDatetime)`. No existing all-day literal or ETag changes.
status: addressed
---
