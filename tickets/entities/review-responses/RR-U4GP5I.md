---
id: RR-U4GP5I
type: review-response
title: rela channel read text output hides choices
finding: 'internal/cli/channel_render.go writeMessageType (:205-235) prints a list property but not the type''s choices. An agent that uses the text output, the default, cannot learn the answer type or into property. Yet docs/realtime.md tells agents to ''post its answer type with the option in into''. Fix: when mt.Choices != nil, print a line such as ''  choices: from options, answer answer into body, other'' through printable, and add a case to TestChannelRender_ListProp.'
severity: minor
resolution: 'writeMessageType prints ''  choices: from <from>, answer <answer> into <into>[, other]'' through printable when the type has choices. Added a subtest to TestChannelRender_ListProp, including an escaped control character, and documented the line under --schemas in GUIDE-cli-reference.'
status: addressed
---
