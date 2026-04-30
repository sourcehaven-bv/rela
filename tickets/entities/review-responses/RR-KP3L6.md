---
id: RR-KP3L6
type: review-response
title: useConfirm unmount-while-pending test is not honest about how it would be exercised
finding: onBeforeUnmount only fires inside an active component setup; you cannot test it by calling useConfirm() at module scope. Plan must either (a) mount a tiny host component in the test (mount(defineComponent({ setup(){...} })) then unmount), or (b) extract the cleanup into a function the test calls directly while the composable wires it via onBeforeUnmount. Pick one explicitly.
severity: significant
resolution: 'Test plan updated: unmount-while-pending will be exercised by mounting a tiny host component (defineComponent({ setup() { exposed = useConfirm(); ... } })) and calling wrapper.unmount(). Vue Test Utils standard pattern.'
status: addressed
---
