---
id: RR-OHFORT
type: review-response
title: There is no logout reset to hook into
finding: The security section says the singleton "is reset on logout with the other stores". No such reset exists. Sign out and switching organisation are full-page links (AccountMenu.vue:93,98), which drop all module state anyway. Correct the sentence so nobody builds a reset hook later.
severity: nit
resolution: 'Corrected the security text: sign out and organisation switches are full-page navigations that drop all state, so no reset hook exists or is needed.'
status: addressed
---
