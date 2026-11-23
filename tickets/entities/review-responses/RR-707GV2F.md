---
id: RR-707GV2F
type: review-response
title: Runs pinned to an older flow version lose their step form
finding: 'internal/appbuild/stepforms.go resolves only forms the current flows name, but Explain/complete use x.Form from the run''s pinned version. If a state moves from form collect_v1 to collect_v2 while data-entry.yaml still declares collect_v1, in-flight runs get fields(collect_v1)=nil: the SPA renders every field read-only and Complete sends {} and advances. Fix: resolve any declared form per lookup against the flow subject, not only those the current flows name.'
severity: significant
resolution: A step resolves the form its state names directly from data-entry.yaml against the subject type of the flow version the run follows; it no longer uses a map of the forms the current flows name. Pinned by TestStepFormSet_Fields.
status: addressed
---
