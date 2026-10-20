---
id: RR-W0QI31
type: review-response
title: Plain Enter in a single-line field submits the composer
finding: 'RlChannelComposer.vue:230-238. The form has a type=submit button, so pressing Enter in any RlTextField (string, integer, ref fallback) triggers implicit submission. In a multi-field card (decision: summary, severity, owner, …) Enter sends a half-filled message whenever only optional fields remain. The hint and the header comment say Ctrl+Enter is the send key, which matches RlCommentComposerBox, where a textarea prevents implicit submit. Fix: handle @keydown.enter on non-textarea inputs with preventDefault, or drop @submit and submit only from the button click and the Ctrl/Meta+Enter handlers. Add a story that presses Enter in a text field and asserts no post.'
severity: minor
resolution: The Send button is no longer a submit button and the form only prevents submission, so Enter in a one-line field does not send (story EnterDoesNotSend).
status: addressed
---
