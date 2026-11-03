---
id: RR-LHZQL9
type: review-response
title: Wire field goldens are missing from the file list
finding: The plan mentions OpenAPI goldens only. internal/dataentry/channels_wire_golden_test.go pins internal/dataentry/testdata/channel_wire_fields.json (which lists ChannelPropertySchema and ChannelMessageTypeSchema fields) and channel_wire.json. New `list`, `max_items` and `choices` fields fail that test. Add both files.
severity: minor
resolution: channel_wire_fields.json and channel_wire.json are in the file list and AC 6 names channels_wire_golden_test.go.
status: addressed
---
