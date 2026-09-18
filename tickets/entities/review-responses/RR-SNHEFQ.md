---
id: RR-SNHEFQ
type: review-response
title: 'Leaked document listener: unmount() unreachable when an assertion fails'
finding: |-
    Both behavioural test files called `wrapper.unmount()` as the last statement of each test body. Vitest throws on a failed `expect`, so that unmount is unreachable EXACTLY when a test fails. The Sidebar/SearchView listeners are bound to `document`, which `document.body.innerHTML = ''` does not clear, so one genuine failure leaks a live handler into every later test in the file.

    The cascade is the real damage: a leaked listener makes `/` fire more than once per press, and inverts the `unregisters its handler on unmount` test — which then fails pointing at teardown logic that is correct. A reader chases a phantom bug while the real failure sits two tests above.
severity: significant
resolution: |-
    Both files now track the wrapper in a `let wrapper: VueWrapper | null` and unmount in `afterEach`, which runs regardless of assertion outcome, plus `vi.restoreAllMocks()` (also covers the separate spy-hygiene finding). Trailing per-test unmounts removed; the unmount test nulls the ref after unmounting so afterEach does not double-unmount.

    Added a positive pin so the contract cannot silently rot: Sidebar's `registers exactly one handler per mount` and SearchView's `toHaveBeenCalledTimes(1)` both fail with a COUNT if a listener ever leaks again — a clear signal instead of an inverted unrelated test.
status: addressed
---

Finding 1 from the cranky-code-reviewer. Verified: the reviewer measured 3
`router.push` calls with two leaked Sidebars plus one live one.
