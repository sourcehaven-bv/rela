---
id: RR-HYVCWA
type: review-response
title: Attacker-controlled newlines escape the target section in append_section
finding: '[security] payload.interpolate (webhook_routes.go:460-461) yields a string that may contain arbitrary newlines, and AppendToSection (markdown/section.go:50-80) splices it in as raw lines. The operator writes a single-line template (''- {{body.msg}}'') and reasonably expects one line; the payload decides otherwise. VERIFIED: msg = ''evil\n---\ntitle: pwned\n---\n\n## Injected\n\n<script>alert(1)</script>'' produced a stored body where the injected ''## Injected'' is a SIBLING of the target ''## Notifications'' section, so subsequent deliveries targeting Notifications now append above it — the payload has reshaped the document structure it was scoped to. Raw HTML is also persisted for whatever renderer consumes it. Two things checked and explicitly NOT findings: the ''---'' does not corrupt frontmatter (store write->read round-trip preserves properties exactly, delimiter stays in the body), and property values with newlines are YAML-serialized safely (no injected keys). Not claiming a live XSS — mailrender sanitizes with bluemonday, but export/SPA/document-render sinks are outside this diff and were not all confirmed. Fix: neutralize line structure at the splice sink in applySteps, where the destination context is known to be ''one markdown line inside a named section'' — collapse or escape CR/LF/NUL before calling AppendToSection. Same posture as internal/mail''s CR/LF rejection on caller-supplied header values. Keep it in dataentry, not in markdown.AppendToSection, whose other callers may legitimately want multi-line input.'
severity: minor
resolution: 'Added flattenToLine in internal/dataentry, applied to the interpolated value before it reaches markdown.AppendToSection: CR and LF become spaces, NUL is dropped. Deliberately flattens rather than rejects — refusing would discard an alert whose producer will not resend it, which is the worse failure for this pipeline. Placed at the splice site rather than inside AppendToSection because that is where the destination context (one line inside a named section) is known; AppendToSection is a general utility whose other callers may legitimately want multi-line input. Same posture as internal/mail validating CR/LF in caller-supplied header values rather than expecting the SMTP library to. Covered by TestWebhookRoutes_AppendSectionFlattensNewlines, which asserts both that the payload''s ''## Injected'' does not become a real heading AND that the text survives — flattened, not dropped.'
status: addressed
---

[security] See `finding`.

Severity is `minor` rather than higher because two plausible escalations were
checked and **do not** hold — worth recording so they are not re-investigated:

- The `---` does **not** corrupt frontmatter. A store write→read round-trip
preserves properties exactly; the delimiter stays in the body.
- Property values containing newlines are YAML-serialized safely — no injected
keys.

What remains real is structural: the injected heading becomes a *sibling* of the
target section, so subsequent deliveries append in the wrong place, and raw HTML
is persisted for downstream renderers.

The fix belongs at the splice site in `applySteps`, not in
`markdown.AppendToSection` — the latter is a general utility whose other callers
may legitimately want multi-line input. `internal/dataentry` is where the
destination context ("one markdown line inside a named section") is known.
