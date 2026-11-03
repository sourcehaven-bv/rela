---
id: RR-CHEFAN
type: review-response
title: Typing line always reserves a blank row and a live region, even for types without typing
finding: 'frontend/packages/rela-components/src/components/channel/RlChannel.vue:345 renders <p class=rl-channel__typing aria-live=polite> unconditionally with min-height: 1lh (:426). Every channel, including types with typing: false and channels without a composer (closed, archived, read-only), gains an empty line between the signals and the composer, and screen readers get an extra live region. Fix: add a prop such as typingEnabled (OpenChannel passes channelType.typing && head._actions.typing or just channelType.typing) and render the region only then; keep it mounted while enabled so the live region exists before its first text.'
severity: minor
resolution: RlChannel renders the typing line only with the new typingOffered prop and a composer; the TypingLineOnlyWhereOffered story checks both cases.
status: addressed
---
