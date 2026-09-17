---
id: RR-VR84MY
type: review-response
title: sanitizeLinkHref returns unnormalized bytes; plan relied on a property it lacks
finding: sanitizeLinkHref normalizes only to decide the scheme and returns the original string, so zero-width/control characters survive into the stored markdown and a leading zero-width is not refused. The plan's validation mechanism relied on a normalization property the function does not have.
severity: critical
resolution: normalizeLinkUrl strips ignored characters itself and normalizes via new URL(), storing url.href. The preset sanitizer is no longer imported (it is also absent from the public .d.ts). Test must assert the returned string is clean, not merely that bad input is refused.
status: addressed
---

**Finding (design review of PLAN-RY25IP).** The plan's URL-validation mechanism
was "compose the preset's exported `sanitizeLinkHref`, it already strips the
characters browsers ignore". Reading the installed source
(`@milkdown/preset-commonmark/lib/index.js:309-317`) shows it normalizes only to
DECIDE the scheme and then returns the original `trimmed` string; the stripped
form is a local that is discarded.

Measured consequences:

- `sanitizeLinkHref('https://ex.com/<ZW>path')` returns the value with the
zero-width character intact, so those bytes would serialize into the entity
markdown — the exact threat the gate exists to stop.
- A leading zero-width (`<ZW>https://ex.com/a`) is not refused at all.
- Obfuscated SCHEMES (`ja<ZW>vascript:`, `java\tscript:`) ARE refused, so there
was no XSS bypass. But the safety was incidental: it came from the function's
internal normalization, not from anything the plan could rely on. A future
change to that return value would flip it to unsafe with no test noticing.

**Resolution.** `normalizeLinkUrl` no longer delegates. It strips the ignored
characters itself, then normalizes through `new URL()` and stores `url.href`,
which is the normalized form. The preset's sanitizer is not imported at all — it
is also absent from the package's public `.d.ts`, so importing it would have
cost a `@ts-expect-error`. It remains defence in depth on the render side, where
the preset calls it for us.

A test must assert the RETURNED string contains no ignored characters, not
merely that bad input is refused — the latter passes today for the wrong reason.
