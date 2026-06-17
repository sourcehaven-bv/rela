---
id: BUG-SQSV6V
type: bug
title: Frontend unit tests fail under happy-dom + DOMPurify >= 3.4.6 (markdown.test.ts strips adjacent <p> tags)
description: |-
    After the dompurify 3.4.5 -> 3.4.10 bump (PR #1000) the `Frontend` CI job went red: `src/utils/markdown.test.ts` fails 8 assertions (e.g. 'separates paragraphs split by a blank line' expects 2 `<p>` but gets 1). `renderMarkdown` sanitizes through DOMPurify, and under the suite's default `happy-dom` test environment DOMPurify >= 3.4.6 mis-serializes adjacent block elements: for raw HTML `<p>first</p>\n<p>second</p>` it emits `first\n<p>second</p>` (the first `<p>`'s tags are stripped). The behavior is environment-specific to happy-dom's DOM serialization — real browsers render correctly, which is why the E2E job stayed green and local runs with a stale dompurify 3.4.5 passed. This blocked all PRs because `Build` (and downstream `Demos`/`Docs`) depend on `Frontend`.

    **Fix:** opt `markdown.test.ts` into the `jsdom` environment via a `// @vitest-environment jsdom` directive (jsdom matches browser DOM serialization; verified all 49 file tests pass under it, 1068/1068 suite-wide). Added `jsdom` as a frontend devDependency. Scoped to the one file that exercises DOMPurify block-serialization rather than switching the whole suite off happy-dom.
priority: medium
effort: s
why1: '`markdown.test.ts` assertions on rendered HTML structure (paragraph/list counts, <br> placement) failed: DOMPurify returned mangled HTML missing the first of two sibling <p> tags.'
why2: DOMPurify >= 3.4.6 serializes/sanitizes adjacent block elements differently under happy-dom, dropping tags that real browsers keep.
why3: 'The frontend unit suite runs under happy-dom (vitest.config.ts `environment: ''happy-dom''`), whose DOM serialization is not byte-compatible with browsers for this DOMPurify path.'
why4: renderMarkdown's correctness depends on browser-accurate DOM serialization (DOMPurify parses+reserializes), an assumption happy-dom silently violated only after the DOMPurify minor bump.
why5: A transitive/dev dependency's runtime behavior under a non-browser test DOM was never pinned or guarded; the auto-merged Dependabot bump (#1000) shipped to develop without the suite catching the env-specific divergence before merge (its own CI was already red for the same reason).
prevention: 'Pin the DOMPurify-dependent test file to jsdom via `// @vitest-environment jsdom`. For any future test that asserts on DOMPurify/serialized-HTML output, prefer jsdom over happy-dom. Longer-term option (not done here to keep the change minimal): evaluate moving the whole suite to jsdom or adding a happy-dom serialization conformance check.'
status: in-progress
---
