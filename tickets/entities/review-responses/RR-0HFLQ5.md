---
id: RR-0HFLQ5
type: review-response
title: placeholder observed but silently no-op after mount
finding: observedAttributes included 'placeholder' but attributeChangedCallback did nothing with it (EasyMDE has no live setter). Observing it invited a 'set it reactively, works at mount, silently fails after' footgun.
severity: significant
resolution: Removed 'placeholder' from observedAttributes so it's read-once-at-mount with no callback inviting the misconception. Documented as mount-time-only.
status: addressed
---
