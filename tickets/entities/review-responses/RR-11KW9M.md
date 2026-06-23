---
id: RR-11KW9M
type: review-response
title: Parse-failure / truncated NOTIFY waits up to 30s for recovery (no immediate catch-up)
finding: 'cranky #2: handleNotification logs Debug + returns on unparseable payload WITHOUT triggering a catch-up. The doc claims ''next catch-up reconciles'' but a live notification doesn''t reset the timeout, so a corrupted/truncated notification is invisible for up to catchUpInterval (30s prod). Realistic cause: pg_notify''s 8000-byte payload cap. ValidateID has no length bound, so a relation with long endpoints (from+reltype+to) could exceed 8000, truncate mid-field, fail the 7-field parse, and silently wait 30s. Fix: trigger an immediate catch-up on parse failure (route a signal from handleNotification back to run), dropping recovery from 30s to ms; document the 8000-byte ceiling.'
severity: significant
resolution: 'Fixed: handleNotification returns needCatchUp bool; the run loop runs an immediate catchUp when an unparseable (e.g. truncated) payload arrives, instead of waiting up to the ticker. Documented the 8000-byte pg_notify ceiling in the feed.go codec doc. New test TestMalformedNotificationTriggersCatchUp: a garbage NOTIFY + a directly-inserted row -> row recovered promptly (with a 1h ticker, so recovery is provably via the immediate catch-up).'
status: addressed
---

## Resolution

handleNotification returns a bool ("needs catch-up") for parse failures / self
having-no-effect; the run inner loop runs an immediate catchUp when it's set,
instead of waiting for the ticker. Also document the 8000-byte NOTIFY ceiling
and that oversized payloads degrade to catch-up. Gives a deterministic test seam
too (feed garbage -> assert immediate catch-up). Also fixes the lack of a
unit-level trigger for the recovery path.
