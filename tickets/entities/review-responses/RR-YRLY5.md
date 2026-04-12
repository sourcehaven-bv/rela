---
id: RR-YRLY5
type: review-response
title: "v-model null coercion risk on month select"
finding: |
  v-model on native select with :value="null" can serialize to string "null" in some browsers,
  bypassing the truthiness guard.
severity: significant
status: addressed
resolution: Replaced v-model with explicit :value and @change handler using empty string as sentinel.
---
