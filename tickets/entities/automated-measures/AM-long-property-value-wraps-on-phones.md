---
id: AM-long-property-value-wraps-on-phones
type: automated-measure
title: Long property value wraps on a phone
description: An e2e test on the phone viewport pins that a long unbroken value stays inside a display and an inline-edit properties section and that the pane does not scroll sideways.
kind: test
location: e2e/tests/mobile-layout.spec.ts
status: active
---

Fails when a long value without spaces runs past a properties section on a
phone.
