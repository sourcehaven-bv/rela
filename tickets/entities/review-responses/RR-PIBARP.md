---
id: RR-PIBARP
type: review-response
title: SVG logos are script-capable and unsupported by mail clients — must not be CID-embedded
finding: The plan correctly discovers the logo must be CID-embedded rather than linked (it sits behind the auth gate), but SVG is in allowedLogoExts (theme_logo.go:24-28). SVG in email has near-zero client support (Gmail, Outlook, Apple Mail strip or fail it) AND is an active-content format that can carry <script>. Embedding operator-uploaded SVG bytes as a CID part ships script-capable content into inboxes. The plan's edge-case line covers 'absent / oversized / unknown ext' but not 'known ext that is unsuitable for email'.
severity: significant
resolution: Plan now restricts the CID-embedded logo to raster formats (png/jpeg/webp) with an explicit allowlist; SVG is skipped and the mail renders without a logo rather than embedding active content. Added to the edge-case list and to the negative tests.
status: addressed
---
