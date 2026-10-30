---
id: RR-4JGM3J
type: review-response
title: 'A preset''s confirm: true quietly drops the type''s confirm text; not documented'
finding: 'buttonoptions.go:62-67 (inherit) moves ConfirmText together with Confirm, so a preset with confirm: true under decision_button: { confirm: "Runs the tool." } shows the translated generic body and not the type''s text. The test pins it (buttonoptions_test.go ''a preset''s confirm true does not take the type''s text'') and the notes give the reason. The guide (GUIDE-realtime.md:1006-1009) only says a preset takes each key it does not set. An operator who writes confirm: true to make it explicit loses the text without warning. Fix: add to that bullet: the text goes with confirm, so confirm: true on a preset shows the standard question, not the type''s text; leave confirm out to keep it.'
severity: minor
resolution: 'GUIDE-realtime Button options: the confirm text goes with confirm, so confirm: true on a preset shows the standard question; leave confirm out to keep the type''s text.'
status: addressed
---
