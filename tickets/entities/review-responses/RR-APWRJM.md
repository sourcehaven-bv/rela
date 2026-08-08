---
id: RR-APWRJM
type: review-response
title: Date inputs rendered blank over stored values (pre-existing); confirm dialog collapsed its field list into a run-on paragraph
finding: |-
    Two display defects surfaced by the demo.

    1. PRE-EXISTING, not caused by this ticket: DateWidget bound the raw stored string to <input type="date">, which accepts ONLY YYYY-MM-DD and silently renders blank for anything else. The API returns RFC3339 for date properties (2026-09-15T00:00:00Z), so every date field showed EMPTY over a perfectly good stored value — on a plain page load, no toggling involved. Confirmed against a freshly loaded form. Especially bad in this ticket's context: an empty date input is exactly what 'my data was destroyed' looks like. The existing unit test only ever passed an already-normalized YYYY-MM-DD, which is how it slipped through. DatetimeWidget already converted before binding (utcISOToLocalInput); DateWidget was the outlier.

    2. ConfirmModal renders `message` as plain text in a <p>, so the \n-separated field list collapsed into one run-on paragraph: 'will clear: • Contract value — policy: confirm: € 2.400.000 • Award criteria ...'. Unreadable, and the point of the dialog is that the user can see exactly what they are about to lose.
severity: significant
resolution: |-
    1. DateWidget now narrows an RFC3339 timestamp to its date part before binding, passing through values already in YYYY-MM-DD and leaving un-parseable input alone (never swallow in-progress typing). Four new unit tests: RFC3339, offset timestamps, un-parseable passthrough, empty.

    2. ConfirmModal's message paragraph gets `white-space: pre-line`, so any caller can present a list without it collapsing. Also rewrote the prompt copy: shorter title that states the count ('Clear these 3 fields?'), one line per field as 'Label — value', and an explicit 'Cancel keeps them and undoes your change' so the non-obvious revert behavior is stated.
status: addressed
---
