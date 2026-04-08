---
id: RR-M5LD
type: review-response
title: No debouncing for text input filters
finding: Plan dismisses debouncing because router.replace doesn't pollute history. But every keystroke also serializes, navigates, parses, and hits the backend. Type 'todo' → 4 backend requests in 200ms. Need 200-300ms debounce around text widget filter changes.
severity: significant
resolution: Added 250ms debounce on text widget filter changes via useDebouncedFn composable (or lodash.debounce if available). Select/multi-select stay immediate.
status: addressed
---
