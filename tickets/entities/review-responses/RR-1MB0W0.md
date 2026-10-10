---
id: RR-1MB0W0
type: review-response
title: Override equal to the config default pins the column
finding: 'setCollapsed always wrote the override, so toggling back left {done:false} and a later collapsed: true default never reached that reader.'
severity: significant
resolution: 'setCollapsed now deletes an override that equals the default and removes the storage key when no overrides remain. Test: drops an override that matches the default, so a new default applies.'
status: addressed
---
