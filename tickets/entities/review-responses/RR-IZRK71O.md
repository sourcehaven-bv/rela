---
id: RR-IZRK71O
type: review-response
title: startSent can fire on an unrelated send
finding: OpenChannel clears startedType only on a matching send or channel switch. Discarding the started form and later posting the same type closes the drawer as if the started decision were sent.
severity: minor
resolution: 'A watcher clears startedType when the form decision ends, and onPost checks the form is still the started type. Test: a send after the started form ended is not the start.'
status: addressed
---
