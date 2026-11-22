---
id: AM-space-prefixed-link-hrefs
type: automated-measure
title: 'Test: links rendered inside a space carry the space'
description: Guards BUG-4AA6RDY. The router.resolve wrapper tests pin the prefix for string, object, named and relative locations, and that push still meets the guard. Table cell, markdown and channel message tests assert /s/<space>/ hrefs.
kind: test
location: frontend/src/router/spaceLinks.test.ts; frontend/src/components/entity/SectionTableCellContent.test.ts; frontend/src/utils/markdown.test.ts; frontend/src/utils/markdown.message.test.ts
status: active
---
