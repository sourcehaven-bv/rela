---
id: markdown-test-jsdom-env
type: automated-measure
title: 'Measure: markdown.test.ts pinned to jsdom for DOMPurify serialization fidelity'
description: Control for BUG-SQSV6V. The renderMarkdown tests run under jsdom instead of the suite-default happy-dom because DOMPurify >= 3.4.6 mis-serializes adjacent block elements under happy-dom (strips the first of two sibling <p> tags) while jsdom and real browsers do not. 49 file tests pass under jsdom; 1068/1068 suite-wide. Guards against env-specific DOMPurify regressions in the markdown-rendering path going undetected by the unit suite.
kind: test
location: frontend/src/utils/markdown.test.ts (// @vitest-environment jsdom directive); frontend/package.json (jsdom devDependency)
status: active
---
