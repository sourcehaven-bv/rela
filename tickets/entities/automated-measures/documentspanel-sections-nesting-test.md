---
id: documentspanel-sections-nesting-test
type: automated-measure
title: Frontend test asserting DocumentsPanel nests inside .sections
description: No test currently asserts DocumentsPanel is a child of div.sections in EntityDetail.vue; a future refactor could silently move it back out to a sibling position and lose the flex gap again with no red test to catch it. Add a frontend component test (or Playwright DOM assertion) that asserts DocumentsPanel renders as a descendant of .sections.
kind: test
location: frontend/src/components/entity/EntityDetail.vue (test not yet added)
status: proposed
---
