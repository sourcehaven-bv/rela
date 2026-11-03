---
id: RR-095QXE
type: review-response
title: Two i18n strings cannot be built as specified
finding: nav.inboxStatus ("{waiting} waiting, {mentions} mentions" with plural forms) has two numbers that each need their own plural. One vue-i18n plural message picks a single form, so build the label from two keys (nav.inboxWaiting with n, nav.inboxMentions with n) and join them. inboxShowing({shown, total}) takes a number total, so it cannot render the "Showing 50 of 20000+" edge case; add a truncated argument or pass total as a preformatted string. For nl, "{author} heeft je genoemd" and "{author} heeft op je gereageerd" read more naturally than "noemde jou" and "reageerde op jou". Give an nl plural pair for nav.inboxStatus too, not only for inboxUnread.
severity: minor
resolution: Split the badge label into nav.inboxWaiting and nav.inboxMentions plural keys joined in code, passed inboxShowing total as a preformatted string for 20000+, used the suggested nl wording, and gave nl plural pairs for every plural key.
status: addressed
---
