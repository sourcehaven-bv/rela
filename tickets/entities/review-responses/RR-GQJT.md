---
id: RR-GQJT
type: review-response
title: Renderer must produce parseable output for mixed lists
finding: 'When rendering a mixed list, the output must be re-parseable. If first item is plain and later items are tasks, goldmark won''t re-parse them as tasks. Need explicit policy: always emit checkbox syntax for task=true items.'
severity: significant
resolution: 'Plan specifies: renderer always emits ''- [x]''/''- [ ]'' for task=true items. Round-trip stability test added; cases that don''t stably round-trip are documented as limitations.'
status: addressed
---
