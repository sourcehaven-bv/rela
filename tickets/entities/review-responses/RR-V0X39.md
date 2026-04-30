---
id: RR-V0X39
type: review-response
title: cmd.label as both modal title AND confirm-button label will overflow for long labels
finding: 'Some commands have long labels (''Generate release notes for the current scope''). Reusing cmd.label as both the modal title and the confirm button label breaks the layout. Match the existing pattern at EntityList.vue:803 (bulk-action modal): title = ''<cmd.label>?'' and confirm button = cmd.label OR title = ''Run command?'' with confirm button = ''Run''. Pick the first — it matches existing UX in this codebase.'
severity: significant
resolution: CommandModal will set title = '<cmd.label>?' and confirmLabel = cmd.label, matching the existing bulk-action pattern at EntityList.vue:803.
status: addressed
---
