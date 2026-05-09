---
id: RR-YWWAL
type: review-response
title: Unmount-while-open leaks pending timer and AbortController
finding: 'watch(() => props.open, ...) only fires cancelInflight() on the open: true → false transition. If the component unmounts while open, the watcher is auto-stopped without ever running its cleanup branch. Fine in production today (App.vue mounts unconditionally) but a real issue under HMR and sets the next maintainer up to fail. Fix: add onBeforeUnmount(() => { cancelInflight(); previouslyFocused.value = null }) to clean up timer + abort controller on teardown.'
severity: critical
resolution: Added onBeforeUnmount(() => { cancelInflight(); previouslyFocused.value = null }). New test 'cancels in-flight request and timer on unmount' verifies the AbortSignal flips to aborted=true on wrapper.unmount() while a request is in flight.
status: addressed
---
