---
id: RR-MR6UMK
type: review-response
title: Heading levels are hardcoded and inconsistent
finding: 'RlChannel.vue:155 uses h2 and RlEntityChannels.vue:53 uses h3. D7 mounts both inside an entity page section below the comments panel, where the right level depends on the host. An h2 inside an h3-level section breaks the outline for screen-reader heading navigation. Fix: take a `headingLevel` prop (or render the title in a slot), as other library panels do. In RlEntityChannels, use aria-labelledby on the section instead of repeating the label in aria-label.'
severity: minor
resolution: RlChannel and RlEntityChannels now take a headingLevel prop (default 3) and render RlHeading, and their sections use aria-labelledby.
status: addressed
---
