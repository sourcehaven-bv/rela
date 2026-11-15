---
id: AM-slash-guard-shell-list-header
type: automated-measure
title: Slash shortcut defers to a list search box drawn outside .entity-list
description: Builds a list page as the shell does (header with the search box as a sibling of .entity-list) and asserts / does not open /search; stubs getClientRects to assert a hidden box does not count.
kind: test
location: frontend/src/components/common/Sidebar.shortcut.test.ts
status: active
---
