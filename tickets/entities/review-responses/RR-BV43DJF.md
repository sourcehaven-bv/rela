---
id: RR-BV43DJF
type: review-response
title: Unknown cell keys dropped silently
finding: 'A typo such as lable: was dropped by the YAML decoder, and a sequence cell failed with a raw yaml.v3 message.'
severity: minor
resolution: KanbanCellColor.UnmarshalYAML rejects any key other than color and label and any other node kind, with the line number. Guide wording adjusted.
status: addressed
---
